# GoTemp (server)

GoTemp is a multi-inbox, single-user temporary email server you can self-host, it features a [SMTP Server](smtp/smtp.go), an [email parser](smtp/parser.go), a [socket server](api/socket.go) and a [REST API](api/api.go) in which you can access the mailboxes and read the emails.

The database of choice is **SQLite**.

## Building

```sh
go mod install .
go build .
```

## Usage

Configuration is done through environment variables, You can view the .env.example for the available variables.

To be able to use the REST API you'll need an access key, generate one using:

```sh
./gotemp --generate-key
```

The generated key will be saved in the *key.secret* file.

After the key has been generated you can then run the program normally:
```sh
./gotemp
```

The program should now be running, You'll need to pass the generated key in the *Authorization* header in all requests you make to the API.

```
Authorization: bearer 94083a69h866055ef6x9fe216f968446e133...(128 chars)
```

## Client

An user interface is provided at [gotemp-client](https://github.com/mateusrdc/gotemp-client).