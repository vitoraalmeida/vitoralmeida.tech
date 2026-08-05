FROM docker.io/library/golang:1.26.0-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY content/ ./content/
COPY static/ ./static/
COPY templates/ ./templates/

RUN CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /usr/local/bin/sitegen \
      ./cmd/sitegen \
    && /usr/local/bin/sitegen build \
      --content ./content \
      --templates ./templates \
      --static ./static \
      --output ./dist

FROM docker.io/library/nginx:1.28.0-alpine AS runtime

RUN rm /etc/nginx/conf.d/default.conf

COPY nginx.conf /etc/nginx/nginx.conf
COPY --from=build /src/dist/ /usr/share/nginx/html/

USER nginx

EXPOSE 8080

CMD ["nginx", "-g", "daemon off;"]
