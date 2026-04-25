// SPDX-License-Identifier: MIT
package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/client/api"
)

func captureStdoutForTest(t *testing.T, outputFunc func() error) string {
	t.Helper()

	originalStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}

	os.Stdout = writePipe
	runErr := outputFunc()
	writePipe.Close()
	os.Stdout = originalStdout

	if runErr != nil {
		t.Fatalf("running output func: %v", runErr)
	}

	var outputBuffer bytes.Buffer
	if _, err := outputBuffer.ReadFrom(readPipe); err != nil {
		t.Fatalf("reading output: %v", err)
	}

	return outputBuffer.String()
}

func TestOutputSearchResultsAsYAML(t *testing.T) {
	searchResponse := &api.SearchResponse{
		Query: "amateur",
		Results: []api.SearchResult{{
			Title:       "Result Title",
			URL:         "https://example.com/video",
			Duration:    "12:34",
			Views:       "1234",
			Engine:      "pornhub",
			Description: "Example description",
			Tags:        []string{"tag1", "tag2"},
		}},
		Count: 1,
	}

	outputText := captureStdoutForTest(t, func() error {
		return OutputSearchResultsAsYAML(searchResponse)
	})

	if !strings.Contains(outputText, "query: amateur") {
		t.Fatalf("yaml output missing query:\n%s", outputText)
	}
	if !strings.Contains(outputText, "title: Result Title") {
		t.Fatalf("yaml output missing result title:\n%s", outputText)
	}
}

func TestOutputSearchResultsAsCSV(t *testing.T) {
	searchResponse := &api.SearchResponse{
		Results: []api.SearchResult{{
			Title:       "Result Title",
			URL:         "https://example.com/video",
			Duration:    "12:34",
			Views:       "1234",
			Engine:      "pornhub",
			Description: "Example description",
			Tags:        []string{"tag1", "tag2"},
		}},
	}

	outputText := captureStdoutForTest(t, func() error {
		return OutputSearchResultsAsCSV(searchResponse)
	})

	if !strings.Contains(outputText, "title,url,duration,views,engine,description,tags") {
		t.Fatalf("csv output missing header:\n%s", outputText)
	}
	if !strings.Contains(outputText, "Result Title,https://example.com/video,12:34,1234,pornhub,Example description,\"tag1,tag2\"") {
		t.Fatalf("csv output missing result row:\n%s", outputText)
	}
}

func TestOutputEnginesAsCSV(t *testing.T) {
	engineList := []EngineInfo{{
		Name:        "pornhub",
		DisplayName: "PornHub",
		Bang:        "ph",
		Tier:        1,
		Enabled:     true,
		Method:      "html",
		HasPreview:  true,
		HasDownload: false,
	}}

	outputText := captureStdoutForTest(t, func() error {
		return OutputEnginesAsCSV(engineList)
	})

	if !strings.Contains(outputText, "name,display_name,bang,tier,enabled,method,has_preview,has_download") {
		t.Fatalf("csv output missing engines header:\n%s", outputText)
	}
	if !strings.Contains(outputText, "pornhub,PornHub,ph,1,true,html,true,false") {
		t.Fatalf("csv output missing engines row:\n%s", outputText)
	}
}

func TestOutputProbeResultsAsCSV(t *testing.T) {
	probeResults := []EngineProbeResult{{
		Name:           "pornhub",
		DisplayName:    "PornHub",
		Tier:           1,
		Available:      true,
		Enabled:        true,
		ResponseTimeMS: 42,
		ResultCount:    3,
		Capabilities: map[string]interface{}{
			"has_preview": true,
		},
		FieldStats: map[string]int{
			ProbeFieldStatHasThumbnail: 3,
		},
	}}

	outputText := captureStdoutForTest(t, func() error {
		return OutputProbeResultsAsCSV(probeResults)
	})

	if !strings.Contains(outputText, "name,display_name,tier,available,enabled,response_time_ms,result_count,error,capabilities,field_stats") {
		t.Fatalf("csv output missing probe header:\n%s", outputText)
	}
	if !strings.Contains(outputText, "\"{\"\"has_preview\"\":true}\"") {
		t.Fatalf("csv output missing capabilities payload:\n%s", outputText)
	}
	if !strings.Contains(outputText, "\"{\"\"has_thumbnail\"\":3}\"") {
		t.Fatalf("csv output missing field stats payload:\n%s", outputText)
	}
}

func TestOutputBashCompletionScriptIncludesYAMLAndCSVFormats(t *testing.T) {
	outputText := captureStdoutForTest(t, func() error {
		return OutputBashCompletionScript()
	})

	if !strings.Contains(outputText, "json yaml csv table plain") {
		t.Fatalf("bash completion output missing yaml/csv formats:\n%s", outputText)
	}
}
