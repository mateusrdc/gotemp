# GoTemp (server)

GoTemp is a multi-inbox, single-user temporary email server you can self-host, it features a [SMTP server](smtp/smtp.go), an [email parser](smtp/parser.go), a [socket server](api/socket.go), a [REST API](api/api.go) and a [web interface](public/) in which you can access the mailboxes and read the emails.

The database of choice is **SQLite**.

## Building

```sh
go build .
```

## Usage

Running the program starts the API and the WebUI.

Configuration is done through environment variables, You can view the [.env.example](.env.example) file for the available variables.

To start using the WebUI and the API you will need to configure a password, the WebUI will guide you through that, in subsequent uses you will need this password to generate a token in which API calls can be made with:


```
Authorization: bearer eyqNjUnYNyvrLbiNbpCJkNpI99f...
```

## Client

Here are some screenshots of the WebUI:

![screenshot 1](https://i.imgur.com/6QBS3UG.png)
![screenshot 2](https://i.imgur.com/jJqJ3VD.png)
![screenshot 3](https://i.imgur.com/k92YT2A.png)

There is a dark mode too:
![screenshot 4](https://i.imgur.com/SAT4ntW.png)