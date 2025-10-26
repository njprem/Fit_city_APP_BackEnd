pipeline {
    agent {
        node {
            label 'DockerHost'
        }
    }

    environment {
        IMAGE_NAME = 'fitcity-backend'
        CONTAINER_NAME = 'fitcity-backend'
        GOROOT = "/usr/local/go"
        PATH = "${GOROOT}/bin:${PATH}"
        GOTOOLCHAIN = "auto"
        DOCKERFILE  = 'infra/docker/api.Dockerfile'
    }

    stages {
        stage('Env check') {
          steps {
            sh '''
              echo "=== Go toolchain ==="
              go version
              go env | egrep 'GOROOT|GOPATH|GOTOOLCHAIN'
              which go
            '''
          }
        }
        stage('Checkout') {
          steps {
            checkout([$class: 'GitSCM',
              userRemoteConfigs: [[
                url: 'https://github.com/njprem/Fit_city_APP_BackEnd.git',
                credentialsId: 'gh-jenkins-02'
              ]],
            //   branches: [[name: '*/main']],                 // <-- match the real branch
              branches: [[name: '*/Auth_Peemai']],    
              extensions: [[$class: 'CleanBeforeCheckout']]
            ])
          }
        }
        stage('Deps & Test') {
          steps {
            sh '''
              # go.mod should be "go 1.24" (no patch). Optionally:
              # echo 'toolchain go1.24.2' >> go.mod
              go clean -modcache
              go mod tidy
              go test ./...
            '''
          }
        }
        stage('Docker Compose Up') {
            steps {
                sh '''
                    docker compose -f infra/compose.prod.yml build --pull api
                    docker compose -f infra/compose.prod.yml up -d --remove-orphans
                '''
            }
        }
        stage('Run DB Migrations') {
          steps {
            sh '''#!/bin/bash
              set -euo pipefail
              for f in migrations/*.sql; do
                echo "Running $f ..."
                psql "postgres://postgres:postgres@10.0.0.11:5432/fitcity?sslmode=disable" -v ON_ERROR_STOP=1 -f "$f" || true
              done
            '''
          }
        }
    }
}