# Auth-service

## Getting started

Сгенерируйте ключи шифрования для JWT аунтентификации

openssl genrsa -out ./config/certs/private.pem 2048

openssl rsa -in ./config/certsprivate.pem -pubout -out ./config/certspublic.pem