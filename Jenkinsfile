pipeline {
    agent none

    environment {
        PROJECT = 'vidveil'
        ORG = 'apimgr'
        REGISTRY = "ghcr.io/${ORG}/${PROJECT}"
        BINDIR = 'binaries'
        RELDIR = 'releases'
        GOCACHE = '/tmp/go-cache'
        GOMODCACHE = '/tmp/go-mod-cache'
    }

    stages {
        stage('Setup') {
            agent { label 'amd64' }
            steps {
                script {
                    env.VERSION = sh(script: 'cat release.txt 2>/dev/null || echo "0.1.0"', returnStdout: true).trim()
                    env.COMMIT_ID = sh(script: 'git rev-parse --short HEAD', returnStdout: true).trim()
                    env.BUILD_DATE = sh(script: 'date +"%a %b %d, %Y at %H:%M:%S %Z"', returnStdout: true).trim()
                    env.LDFLAGS = "-s -w -X 'main.Version=${env.VERSION}' -X 'main.CommitID=${env.COMMIT_ID}' -X 'main.BuildDate=${env.BUILD_DATE}'"
                }
                sh 'mkdir -p ${BINDIR} ${RELDIR}'
            }
        }

        stage('Build') {
            parallel {
                stage('Linux AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                -v ${WORKSPACE}:/build \
                                -v ${GOCACHE}:/root/.cache/go-build \
                                -v ${GOMODCACHE}:/go/pkg/mod \
                                -w /build \
                                -e CGO_ENABLED=0 \
                                -e GOOS=linux \
                                -e GOARCH=amd64 \
                                golang:alpine \
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT}-linux-amd64 ./src
                        '''
                    }
                }
                stage('Linux ARM64') {
                    agent { label 'arm64' }
                    steps {
                        sh '''
                            docker run --rm \
                                -v ${WORKSPACE}:/build \
                                -v ${GOCACHE}:/root/.cache/go-build \
                                -v ${GOMODCACHE}:/go/pkg/mod \
                                -w /build \
                                -e CGO_ENABLED=0 \
                                -e GOOS=linux \
                                -e GOARCH=arm64 \
                                golang:alpine \
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECT}-linux-arm64 ./src
                        '''
                    }
                }
            }
        }

        stage('Test') {
            agent { label 'amd64' }
            steps {
                sh '''
                    docker run --rm \
                        -v ${WORKSPACE}:/build \
                        -v ${GOCACHE}:/root/.cache/go-build \
                        -v ${GOMODCACHE}:/go/pkg/mod \
                        -w /build \
                        golang:alpine \
                        go test -v -cover ./...
                '''
            }
        }

        stage('Release') {
            agent { label 'amd64' }
            when {
                buildingTag()
            }
            steps {
                sh '''
                    # Create version.txt
                    echo "${VERSION}" > ${RELDIR}/version.txt

                    # Copy and strip binaries
                    for f in ${BINDIR}/${PROJECT}-*; do
                        [ -f "$f" ] || continue
                        strip "$f" 2>/dev/null || true
                        cp "$f" ${RELDIR}/
                    done

                    # Create source archive
                    tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
                        --exclude='binaries' --exclude='releases' --exclude='*.tar.gz' \
                        -czf ${RELDIR}/${PROJECT}-${VERSION}-source.tar.gz .
                '''
                archiveArtifacts artifacts: 'releases/*', fingerprint: true
            }
        }

        stage('Docker') {
            agent { label 'amd64' }
            // Runs on ALL branches and tags
            // Multi-stage Dockerfile handles Go compilation - no pre-built binaries needed
            steps {
                script {
                    def tags = "-t ${REGISTRY}:${COMMIT_ID}"

                    if (env.TAG_NAME) {
                        // Release tag - version, latest, YYMM
                        def yymm = new Date().format('yyMM')
                        tags += " -t ${REGISTRY}:${VERSION}"
                        tags += " -t ${REGISTRY}:latest"
                        tags += " -t ${REGISTRY}:${yymm}"
                    } else {
                        // All branches get devel tag
                        tags += " -t ${REGISTRY}:devel"

                        // Beta branch also gets beta tag
                        if (env.BRANCH_NAME == 'beta') {
                            tags += " -t ${REGISTRY}:beta"
                        }
                    }

                    sh """
                        docker buildx create --name ${PROJECT}-builder --use 2>/dev/null || docker buildx use ${PROJECT}-builder
                        docker buildx build \
                            -f ./docker/Dockerfile \
                            --platform linux/amd64,linux/arm64 \
                            --build-arg VERSION="${VERSION}" \
                            --build-arg COMMIT_ID="${COMMIT_ID}" \
                            --build-arg BUILD_DATE="${BUILD_DATE}" \
                            ${tags} \
                            --push \
                            .
                    """
                }
            }
        }
    }

    post {
        always {
            cleanWs()
        }
    }
}
