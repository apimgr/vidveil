// SPDX-License-Identifier: MIT
package geoip

import (
	"reflect"
	"testing"
)

func TestCountryRecordUsesIPLocationDBField(t *testing.T) {
	field, ok := reflect.TypeOf(countryRecord{}).FieldByName("CountryCode")
	if !ok {
		t.Fatal("countryRecord is missing CountryCode")
	}
	if got := field.Tag.Get("maxminddb"); got != "country_code" {
		t.Errorf("countryRecord.CountryCode tag = %q, want %q", got, "country_code")
	}

	result := &GeoIPResult{}
	applyCountryRecord(result, countryRecord{CountryCode: "GB"})
	if result.CountryCode != "GB" {
		t.Errorf("CountryCode = %q, want GB", result.CountryCode)
	}
}

func TestCityRecordUsesIPLocationDBFlatFields(t *testing.T) {
	type expectedField struct {
		name string
		tag  string
	}
	fields := []expectedField{
		{"City", "city"},
		{"CountryCode", "country_code"},
		{"State1", "state1"},
		{"State2", "state2"},
		{"Postcode", "postcode"},
		{"Latitude", "latitude"},
		{"Longitude", "longitude"},
		{"Timezone", "timezone"},
	}
	typ := reflect.TypeOf(cityRecord{})
	for _, expected := range fields {
		field, ok := typ.FieldByName(expected.name)
		if !ok {
			t.Errorf("cityRecord is missing %s", expected.name)
			continue
		}
		if got := field.Tag.Get("maxminddb"); got != expected.tag {
			t.Errorf("cityRecord.%s tag = %q, want %q", expected.name, got, expected.tag)
		}
	}
}

func TestApplyCityRecordMapsFlatFields(t *testing.T) {
	result := &GeoIPResult{}
	applyCityRecord(result, cityRecord{
		City:        "London",
		CountryCode: "GB",
		State1:      "England",
		State2:      "Greater London",
		Postcode:    "SW1A",
		Latitude:    51.501,
		Longitude:   -0.142,
		Timezone:    "Europe/London",
	})

	want := GeoIPResult{
		CountryCode: "GB",
		City:        "London",
		Region:      "England",
		Postal:      "SW1A",
		Latitude:    51.501,
		Longitude:   -0.142,
		Timezone:    "Europe/London",
	}
	if *result != want {
		t.Errorf("mapped GeoIPResult = %+v, want %+v", *result, want)
	}
}

func TestApplyCityRecordUsesSecondStateWhenFirstIsEmpty(t *testing.T) {
	result := &GeoIPResult{}
	applyCityRecord(result, cityRecord{State2: "Region"})
	if result.Region != "Region" {
		t.Errorf("Region = %q, want Region", result.Region)
	}
}
