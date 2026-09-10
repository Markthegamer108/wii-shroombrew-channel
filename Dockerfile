# ---- build stage ----
FROM golang:1.21-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/shroombrew .

# ---- run stage ----
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/shroombrew /app/shroombrew
COPY config.example.xml /app/config.example.xml
COPY database.sql /app/database.sql
EXPOSE 8080
ENTRYPOINT ["/app/shroombrew"]
