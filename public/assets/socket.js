class SocketInterface {
    constructor(url, token) {
        const protocol = new URL(url).protocol === "https:" ? "wss:" : "ws:";

        this.url = protocol + "//" + url.substr(url.indexOf("//") + 2);
        this.token = token;
        this.suppressCloseMessage = false;
        this.pingInterval = null;

        this.connect();
    }

    connect() {
        this.socket = new WebSocket(`${this.url}/socket`)

        this.socket.addEventListener("open", () => this.auth());
        this.socket.addEventListener("message", (...args) => this.onMessage(...args));
        this.socket.addEventListener("close", () => this.onClose());
        this.socket.addEventListener("error", () => this.onClose());
    }

    auth() {
        this.socket.send("auth " + this.token);
    }

    sendPing() {
        this.socket.send("ping");
    }

    onMessage(messageData) {
        if (messageData === "pong") return;

        let message;

        try {
            message = JSON.parse(messageData.data);
        } catch (e) {
            return
        }

        switch(message.type) {
            case "AUTH_OK": {
                // Init ping interval
                this.pingInterval = setInterval(this.sendPing.bind(this), 60 * 1000)

                break;
            }
            case "AUTH_ERROR": {
                this.suppressCloseMessage = true;
                notify("Error connecting to socket!", "danger");

                break;
            }
            case "MAILBOX_EDITED": {
                const index = app.mailboxes.findIndex(i => i.id === message.data.id);

                if (index !== -1) {
                    app.$set(app.mailboxes, index, message.data);
                }

                // Edit currentMailbox if we're on it
                if (app.currentMailbox.id === message.data.id) {
                    app.currentMailbox = message.data;
                }

                break;
            }
            case "MAILBOX_CREATED": {
                app.mailboxes.push(message.data);

                break;
            }
            case "MAILBOX_DELETED": {
                const index = app.mailboxes.findIndex(i => i.id === message.data);

                if (index !== -1) {
                    // Go back if we're in this mailbox
                    if (app.state > 1 && app.currentMailbox.id === message.data) {
                        app.goBackToStart();
                    }

                    app.mailboxes.splice(index, 1);
                }

                break;
            }
            case "NEW_EMAIL": {
                const index = app.mailboxes.findIndex(i => i.id === message.data.mailbox_id);

                if (index !== -1) {
                    if (app.state > 1 && app.currentMailbox.id === message.data.mailbox_id) {
                        // Insert new email to the view
                        app.emails.unshift(message.data.email);
                    }

                    app.mailboxes[index].unread_count++;
                    app.mailboxes[index].last_email_at = new Date().toISOString();
                    array_move(app.mailboxes, index, 0);;
                }

                break;
            }
            default:
                console.log("Not handled socket message: ", message);
        }
    }

    onClose() {
        if (this.suppressCloseMessage) return;

        notify("Socket connection closed!", "danger");
    }
}