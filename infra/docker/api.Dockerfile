FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /out/api ./cmd/api

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /src/docs /app/docs
RUN apk add --no-cache ca-certificates ffmpeg && update-ca-certificates
ENV PORT=8080
EXPOSE 8080
CMD ["/app/api"]
