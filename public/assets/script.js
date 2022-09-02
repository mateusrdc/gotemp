const app = new Vue({
    el: "#app",

    data: {
        api: null,
        socket: null,

        server_name: null,
        server_configured: true,
        ready: false,
        state: 1,
        darkMode: false,
        shiftKeyPressed: false,

        mailboxes: [],
        currentMailbox: null,

        emails: [],
        currentEmail: null,
        lastCheckedEmailID: null,
        viewHeaders: false,
        viewInverted: false,

        modalMode: "",
        modalName: "",
        modalAddress: "",
        modalExpiration: "",
        modalMailbox: null,
        modalSaving: false,
    },

    methods: {
        async tryConnect(token) {
            const server_address = getServerAddress();
            const api_address = getApiAddress();
            const result = await checkApiServer(api_address, token)

            if (result) {
                notify("Connection established!", "success");

                this.api = new ApiInterface(api_address, token);
                this.socket = new SocketInterface(server_address, token);
                this.server_name = result.server_name;
                this.server_configured = !result.unconfigured;
                
                if (this.server_configured) {
                    this.ready = true;
                    this.loadMailboxes();
                }

                return true;
            } else {
                if (token !== null) {
                    notify("Error connecting to server!", "danger");
                }

                return false;
            }
        },

        async configureServer() {
            const new_key = document.querySelector("#server_new_key_input").value;

            await this.api.PUT("/key", new_key)
                .then(result => {
                    localStorage.setItem("server_jwt", result.token);
                    this.tryConnect(result.token);
                })
                .catch(e => {
                    notify("Error setting key!");
                })
        },

        async loadMailboxes() {
            this.mailboxes = await this.api.GET("/mailboxes")
                .then(data => {
                    return data.mailboxes;
                })
                .catch(e => {
                    notify("Error loading Mailboxes!");
                });
        },

        async loadMailbox(mbox) {
            this.currentMailbox = mbox;
            this.state = 2;

            // Load emails
            this.emails = await this.api.GET(`/mailboxes/${mbox.id}`)
                .then(data => {
                    return data.mailbox.emails;
                })
                .catch(e => {
                    notify("Error loading Mailbox!", "danger");
                    this.state = 1;

                    return [];
                })
        },

        async loadEmail(email, evt) {
            this.currentEmail = email;
            this.state = 3;

            // Mark email as ready if it isn't already
            if (!email.read) {
                await this.api.PUT(`/mailboxes/${this.currentMailbox.id}/${email.id}/read`)
                    .then(data => {
                        this.currentMailbox.unread_count--;
                        email.read = true;
                    })
                    .catch(e => {
                        console.log("Error marking email as ready", email.id, e);
                    });
            }
        },

        async toggleMailboxLocked(mailbox) {
            const copy = Object.assign({}, mailbox);

            copy.locked = !copy.locked;
            
            await saveEditedMailbox(copy)
        },

        async deleteMailbox(mailbox) {
            await UIkit.modal.confirm('Do you really want to delete this Mailbox?<br/><br/>' + mailbox.name)
                .then(async () => {
                    await this.api.DELETE(`/mailboxes/${mailbox.id}`)
                        .then(data => {
                            // Local state deletion will happen at the socket level
                        })
                        .catch(e => {
                            notify("Error deleting mailbox!", "danger");
                        });
                })
                .catch(e => {});
        },

        async copyMailboxAddress(mailbox) {
            await navigator.clipboard.writeText(mailbox.address + "@" + this.server_name)
        },

        async deleteEmails(mailbox_id, email_list) {
            if (email_list.length > 50) {
                return notify("Can't delete more than 50 emails at once!", "danger");
            }

            if (await _deleteEmails(mailbox_id, email_list)) {
                // Delete mails from local state
                emails_copy = this.emails.slice(0);

                for (const email_id of email_list) {
                    const index = emails_copy.findIndex(e => e.id === email_id);

                    if (index !== -1) {
                        emails_copy.splice(index, 1);
                    }
                }

                this.emails = emails_copy;

                // Update mailbox's local state
                let index = this.mailboxes.findIndex(e => e.id === mailbox_id);

                if (index !== -1) {
                    app.mailboxes[index].unread_count = email_list.length;
                }

                // Go back to the mailbox if we're in one of the deleted emails
                index = email_list.findIndex(e => e.id === this.currentEmail.id)

                if (index !== -1) {
                    this.goBack();
                }
            } else {
                notify("Error deleting email(s)", "danger");
            }
        },

        async deleteSelectedEmails() {
            let email_list = [];

            for (const email of this.emails) {
                if (email._checked) {
                    email_list.push(email.id);
                }
            }

            await this.deleteEmails(this.currentMailbox.id, email_list);
        },

        mailboxEmailCheckboxChanged(email) {
            // Handle shift + click to select multiple emails
            if (email._checked) {
                if (this.lastCheckedEmailID && this.shiftKeyPressed) {
                    const emailIndex = this.emails.findIndex(e => e.id === email.id);
                    const lastCheckedEmailIndex = this.emails.findIndex(e => e.id === this.lastCheckedEmailID);

                    if (emailIndex !== -1 && lastCheckedEmailIndex != -1) {
                        for (let i = Math.min(emailIndex, lastCheckedEmailIndex) + 1 ; i < Math.max(emailIndex, lastCheckedEmailIndex) ; i++) {
                            this.emails[i]._checked = true;
                        }
                    }
                }

                this.lastCheckedEmailID = email.id;
            } else {
                this.lastCheckedEmailID = null;
            }
        },

        openMailboxModal(mode, mailbox) {
            this.modalMode = mode;
            this.modalName = mailbox ? mailbox.name : "";
            this.modalAddress = mailbox ? mailbox.address : "";
            this.modalExpiration = mailbox ? browserFormatDate(new Date(mailbox.expires_at)) : getDefaultModalDate();
            this.modalMailbox = mailbox ?? null;
            this.modalSaving = false;

            UIkit.modal('#mailbox-modal').show();
        },

        async modalSave() {
            this.modalSaving = true;

            const mbox = this.modalMailbox ?? {};

            mbox.name = this.modalName;
            mbox.address = this.modalAddress;
            mbox.expires_at = this.modalExpiration == "never" ? this.modalExpiration : (new Date(this.modalExpiration).toISOString());
            mbox.locked = mbox.locked ?? false;
            mbox.emails = null;

            if (this.modalMode === "create") {
                await this.api.POST("/mailboxes", mbox)
                    .then(data => {
                        UIkit.modal("#mailbox-modal").hide();
                        notify("Mailbox created successfully!");
                    })
                    .catch(data => {
                        notify("Error creating mailbox: " + data.error);
                        this.modalSaving = false;
                    });
            } else if (this.modalMode === "edit") {
                await this.api.PUT("/mailboxes/" + mbox.id, mbox)
                    .then(data => {
                        UIkit.modal("#mailbox-modal").hide();
                        notify("Mailbox edited successfully!");
                    })
                    .catch(data => {
                        notify("Error editing mailbox: " + data.error);
                        this.modalSaving = false;
                    });
            }
        },

        switchTheme() {
            const in_dark_mode = document.body.classList.contains("dark");

            this.darkMode = !in_dark_mode;

            document.body.classList.toggle("dark");
            document.getElementById("app").classList.toggle("uk-light");
            localStorage.setItem("dark_theme", this.darkMode);
        },

        async logout() {
            await UIkit.modal.confirm("Are you sure you want to logout?")
                .then(async () => {
                    localStorage.removeItem("server_jwt");
                    window.location.reload();
                })
                .catch(e => { });
        },

        goBack() {
            if (this.state === 2) {
                this.state = 1;
                this.currentMailbox = null;
                this.emails = [];
                this.lastCheckedEmailID = null;
            } else if (this.state == 3) {
                this.state = 2;
                this.currentEmail = null;
                this.viewHeaders = false;
                this.viewInverted = false;
            }
        },

        goBackToStart() {
            while (this.state != 1) {
                this.goBack();
            }
        }
    },

    computed: {
        currentEmailBody() {
            if (!this.currentEmail) return "";

            if (!this.viewHeaders) {
                return DOMPurify.sanitize(this.currentEmail.body, { WHOLE_DOCUMENT: true, FORCE_BODY: true, ADD_TAGS: ["link"], ADD_ATTR: ["href"] });
            } else {
                return DOMPurify.sanitize(this.currentEmail.headers.replaceAll("\n", "<br/>"), {});
            }
        },

        checkedEmailCount() {
            if (!this.emails) return 0;

            return this.emails.filter(e => e._checked).length;
        }
    },

    async created() {
        // Load dark theme if the user has set it previously
        const dark_theme_status = localStorage.getItem("dark_theme");

        if ((dark_theme_status === "true" && !this.darkMode) || this.darkMode) {
            this.switchTheme();
        }

        // Try to load data from localStorage
        const token = localStorage.getItem("server_jwt")

        await this.tryConnect(token);

        // Load mailboxes if we're ready
        if (this.ready) {
            this.loadMailboxes();
        }

        // Listen for the shift key up/down events
        document.addEventListener("keyup", e => { this.shiftKeyPressed = e.shiftKey });
        document.addEventListener("keydown", e => { this.shiftKeyPressed = e.shiftKey });
    }
})