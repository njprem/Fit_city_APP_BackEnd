FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /out/api ./cmd/api

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/api /app/api
ENV PORT=8080
EXPOSE 8080
CMD ["/app/api"]