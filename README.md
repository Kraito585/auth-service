# Auth-service

## Getting started

Сгенерируйте ключи шифрования для JWT аунтентификации

openssl genrsa -out ./config/certs/private.pem 2048

openssl rsa -in ./config/certsprivate.pem -pubout -out ./config/certspublic.pem

Основон сервер на моём паке для микросервис k8s разработки [go-core-cli](https://github.com/Kraito585/go-core-cli) 

Сервис писался под мои проекты реализация которых отложена в долгий ящик, он довольно безопасен. На сколько он соответсвует нормативам attach2 я не проверял.
