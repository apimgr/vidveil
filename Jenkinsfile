pipeline {
    agent none

    triggers {
        // Daily build at 3am UTC (matches GitHub Actions daily.yml)
        cron('0 3 * * *')
    }

    environment {
        PROJECTNAME = 'vidveil'
        PROJECTORG = 'apimgr'
        BINDIR = 'binaries'
        RELDIR = 'releases'
        GOCACHE = "/tmp/${PROJECTORG}/go-cache"
        GOMODCACHE = "/tmp/${PROJECTORG}/go-mod-cache"

        // Git provider configuration (GitHub default)
        GIT_FQDN = 'github.com'
        REGISTRY = "ghcr.io/${PROJECTORG}/${PROJECTNAME}"
    }

    stages {
        stage('Setup') {
            agent { label 'amd64' }
            steps {
                script {
                    // Determine build type and version
                    if (env.TAG_NAME) {
                        // Release build (tag push)
                        env.BUILD_TYPE = 'release'
                        env.VERSION = env.TAG_NAME.replaceFirst('^v', '')
                    } else if (env.BRANCH_NAME == 'beta') {
                        // Beta build
                        env.BUILD_TYPE = 'beta'
                        env.VERSION = sh(script: 'date -u +"%Y%m%d%H%M%S"', returnStdout: true).trim() + '-beta'
                    } else if (env.BRANCH_NAME == 'main' || env.BRANCH_NAME == 'master') {
                        // Daily build
                        env.BUILD_TYPE = 'daily'
                        env.VERSION = sh(script: 'date -u +"%Y%m%d%H%M%S"', returnStdout: true).trim()
                    } else {
                        // Other branches - dev build
                        env.BUILD_TYPE = 'dev'
                        env.VERSION = sh(script: 'date -u +"%Y%m%d%H%M%S"', returnStdout: true).trim() + '-dev'
                    }
                    env.COMMIT_ID = sh(script: 'git rev-parse --short HEAD', returnStdout: true).trim()
                    env.BUILD_DATE = sh(script: 'date +"%a %b %d, %Y at %H:%M:%S %Z"', returnStdout: true).trim()
                    // OFFICIALSITE from Jenkins credentials or environment
                    env.OFFICIALSITE = env.OFFICIALSITE ?: ''
                    env.LDFLAGS = "-s -w -X 'main.Version=${env.VERSION}' -X 'main.CommitID=${env.COMMIT_ID}' -X 'main.BuildDate=${env.BUILD_DATE}' -X 'main.OfficialSite=${env.OFFICIALSITE}'"
                    env.HAS_CLI = sh(script: '[ -d src/client ] && echo true || echo false', returnStdout: true).trim()
                }
                sh 'mkdir -p ${BINDIR} ${RELDIR}'
                echo "Build type: ${BUILD_TYPE}, Version: ${VERSION}"
            }
        }

        stage('Build Server') {
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
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECTNAME}-linux-amd64 ./src
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
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECTNAME}-linux-arm64 ./src
                        '''
                    }
                }
                stage('Darwin AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                -v ${WORKSPACE}:/build \
                                -v ${GOCACHE}:/root/.cache/go-build \
                                -v ${GOMODCACHE}:/go/pkg/mod \
                                -w /build \
                                -e CGO_ENABLED=0 \
                                -e GOOS=darwin \
                                -e GOARCH=amd64 \
                                golang:alpine \
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECTNAME}-darwin-amd64 ./src
                        '''
                    }
                }
                stage('Darwin ARM64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                -v ${WORKSPACE}:/build \
                                -v ${GOCACHE}:/root/.cache/go-build \
                                -v ${GOMODCACHE}:/go/pkg/mod \
                                -w /build \
                                -e CGO_ENABLED=0 \
                                -e GOOS=darwin \
                                -e GOARCH=arm64 \
                                golang:alpine \
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECTNAME}-darwin-arm64 ./src
                        '''
                    }
                }
                stage('Windows AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                -v ${WORKSPACE}:/build \
                                -v ${GOCACHE}:/root/.cache/go-build \
                                -v ${GOMODCACHE}:/go/pkg/mod \
                                -w /build \
                                -e CGO_ENABLED=0 \
                                -e GOOS=windows \
                                -e GOARCH=amd64 \
                                golang:alpine \
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECTNAME}-windows-amd64.exe ./src
                        '''
                    }
                }
                stage('Windows ARM64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                -v ${WORKSPACE}:/build \
                                -v ${GOCACHE}:/root/.cache/go-build \
                                -v ${GOMODCACHE}:/go/pkg/mod \
                                -w /build \
                                -e CGO_ENABLED=0 \
                                -e GOOS=windows \
                                -e GOARCH=arm64 \
                                golang:alpine \
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECTNAME}-windows-arm64.exe ./src
                        '''
                    }
                }
                stage('FreeBSD AMD64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                -v ${WORKSPACE}:/build \
                                -v ${GOCACHE}:/root/.cache/go-build \
                                -v ${GOMODCACHE}:/go/pkg/mod \
                                -w /build \
                                -e CGO_ENABLED=0 \
                                -e GOOS=freebsd \
                                -e GOARCH=amd64 \
                                golang:alpine \
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECTNAME}-freebsd-amd64 ./src
                        '''
                    }
                }
                stage('FreeBSD ARM64') {
                    agent { label 'amd64' }
                    steps {
                        sh '''
                            docker run --rm \
                                -v ${WORKSPACE}:/build \
                                -v ${GOCACHE}:/root/.cache/go-build \
                                -v ${GOMODCACHE}:/go/pkg/mod \
                                -w /build \
                                -e CGO_ENABLED=0 \
                                -e GOOS=freebsd \
                                -e GOARCH=arm64 \
                                golang:alpine \
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECTNAME}-freebsd-arm64 ./src
                        '''
                    }
                }
            }
        }

        stage('Build CLI') {
            when {
                expression { env.HAS_CLI == 'true' }
            }
            parallel {
                stage('CLI Linux AMD64') {
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
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECTNAME}-linux-amd64-cli ./src/client
                        '''
                    }
                }
                stage('CLI Linux ARM64') {
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
                                go build -ldflags "${LDFLAGS}" -o ${BINDIR}/${PROJECTNAME}-linux-arm64-cli ./src/client
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
                    for f in ${BINDIR}/${PROJECTNAME}-*; do
                        [ -f "$f" ] || continue
                        strip "$f" 2>/dev/null || true
                        cp "$f" ${RELDIR}/
                    done

                    # Create source archive
                    tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
                        --exclude='binaries' --exclude='releases' --exclude='*.tar.gz' \
                        -czf ${RELDIR}/${PROJECTNAME}-${VERSION}-source.tar.gz .
                '''
                archiveArtifacts artifacts: 'releases/*', fingerprint: true
            }
        }

        stage('Docker') {
            agent { label 'amd64' }
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
                        docker buildx create --name ${PROJECTNAME}-builder --use 2>/dev/null || docker buildx use ${PROJECTNAME}-builder
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
