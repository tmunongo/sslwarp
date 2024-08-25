setup_ssl:
	openssl genrsa -out certs/server.key 2048
	openssl req -new -key certs/server.key -out certs/server.csr
	openssl x509 -req -days 365 -in certs/server.csr -signkey certs/server.key -out certs/server.crt
	cat certs/server.crt certs/server.key > certs/server.pem