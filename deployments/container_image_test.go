package deployments

import (
	"os"
	"strings"
	"testing"
)

func TestGoServiceDockerfileBuildsKiwiGuardBinary(t *testing.T) {
	body, err := os.ReadFile("../Dockerfile")
	if err != nil {
		t.Fatalf("ReadFile(../Dockerfile) error = %v", err)
	}

	dockerfile := string(body)
	for _, want := range []string{
		"FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build",
		"CGO_ENABLED=0",
		"go build",
		"./cmd/kiwiguard",
		"FROM gcr.io/distroless/static-debian12:nonroot",
		"COPY --from=build /out/kiwiguard /kiwiguard",
		"USER nonroot:nonroot",
		"ENTRYPOINT [\"/kiwiguard\"]",
		"CMD [\"serve\"]",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("Dockerfile missing %q:\n%s", want, dockerfile)
		}
	}
}

func TestDockerIgnoreKeepsImageBuildContextFocused(t *testing.T) {
	body, err := os.ReadFile("../.dockerignore")
	if err != nil {
		t.Fatalf("ReadFile(../.dockerignore) error = %v", err)
	}

	ignore := string(body)
	for _, want := range []string{
		".git",
		".env",
		"coverage.out",
		"web/node_modules",
		"web/dist",
	} {
		if !strings.Contains(ignore, want) {
			t.Fatalf(".dockerignore missing %q:\n%s", want, ignore)
		}
	}
	for _, required := range []string{"go.mod", "cmd", "internal"} {
		if strings.Contains(ignore, required) {
			t.Fatalf(".dockerignore excludes required build input %q:\n%s", required, ignore)
		}
	}
}

func TestMakefileExposesFormalImageBuildTargets(t *testing.T) {
	body, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatalf("ReadFile(../Makefile) error = %v", err)
	}

	makefile := string(body)
	for _, want := range []string{
		"IMAGE_REPOSITORY ?= ghcr.io/howmuchsec/kiwiguard",
		"IMAGE_TAG ?= dev",
		"IMAGE ?= $(IMAGE_REPOSITORY):$(IMAGE_TAG)",
		"DOCKER_BUILD_PLATFORM ?= linux/amd64",
		"docker-image:",
		"docker image build",
		"--platform $(DOCKER_BUILD_PLATFORM)",
		"--tag $(IMAGE)",
		"docker-image-smoke:",
		"--platform $(DOCKER_BUILD_PLATFORM) $(IMAGE) --help",
	} {
		if !strings.Contains(makefile, want) {
			t.Fatalf("Makefile missing %q:\n%s", want, makefile)
		}
	}
}

func TestMakefileExposesProductionComposeValidationTarget(t *testing.T) {
	body, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatalf("ReadFile(../Makefile) error = %v", err)
	}

	makefile := string(body)
	for _, want := range []string{
		"PROD_ENV_FILE ?= .env.production.example",
		"docker-production-config:",
		"docker compose -f deployments/production-compose.yml --env-file $(PROD_ENV_FILE) config",
	} {
		if !strings.Contains(makefile, want) {
			t.Fatalf("Makefile missing %q:\n%s", want, makefile)
		}
	}
}
