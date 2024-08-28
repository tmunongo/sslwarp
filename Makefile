setup_ssl:
	openssl genrsa -out certs/server.key 2048
	openssl req -new -key certs/server.key -out certs/server.csr
	openssl x509 -req -days 365 -in certs/server.csr -signkey certs/server.key -out certs/server.crt
	cat certs/server.crt certs/server.key > certs/server.pem

make local_server:
	air

make local_client:
	go run cmd/main.go --config ./config.yml

run: build
	@./bin/app

build:
	@go build -o bin/app cmd/main.go


css:
	tailwindcss -i views/css/app.css -o public/styles.css --watch

templ:
	templ generate --watch --proxy=http://localhost:4000

prod:
	