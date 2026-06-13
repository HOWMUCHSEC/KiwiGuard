# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
		-trimpath \
		-ldflags="-s -w" \
		-o /out/kiwiguard \
		./cmd/kiwiguard

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="KiwiGuard"
LABEL org.opencontainers.image.description="OpenAI-compatible LLM guardrail gateway"
LABEL org.opencontainers.image.source="https://github.com/howmuchsec/kiwiguard"

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/kiwiguard /kiwiguard

USER nonroot:nonroot
EXPOSE 8080 8081

ENTRYPOINT ["/kiwiguard"]
CMD ["serve"]
