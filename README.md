# Backer

**Backer** is a crowdfunding platform backend that connects campaign creators with donors. It handles everything needed to run a donation-based platform — from user authentication and campaign management to payment processing and transaction tracking. It also ships with a session-based **Admin CMS** for managing users, campaigns, and transactions from the browser.

## About the Project

Backer allows users to:

- Create an account and log in securely
- Launch and manage fundraising campaigns, with cover image uploads
- Receive donations from backers/donors
- Process payments through an integrated payment gateway (Midtrans)
- Track and review transaction history

It also includes an **Admin CMS**, protected by session-based login (separate from the API's JWT auth), where admins can:

- Manage users, including avatar uploads
- Manage campaigns: create, edit, view detail, and upload campaign images
- View a list of all transactions with amounts formatted in Rupiah (IDR)

Built entirely in **Go**, the project follows a clean, modular architecture separating each domain (auth, campaign, payment, transaction, user) into its own package — making it easy to maintain and extend.

## Tech Stack

- **Language:** Go (Golang)
- **Web framework:** [Gin](https://github.com/gin-gonic/gin)
- **ORM:** [GORM](https://gorm.io/) with MySQL
- **Auth:** JWT (REST API) and cookie-based sessions (Admin CMS)
- **Payment gateway:** [Midtrans](https://midtrans.com/)
- **Templating:** Go `html/template` with [gin-contrib/multitemplate](https://github.com/gin-contrib/multitemplate) for the Admin CMS
- **Architecture:** Modular, domain-driven structure

## Project Structure

```
backer-backend/
├── auth/          # Authentication & authorization logic (JWT)
├── campaign/      # Campaign domain (model, service, repository, formatter)
├── config/        # App configuration (reads values from .env)
├── handler/       # HTTP handlers for the REST API
├── helper/        # Utility functions & response formatting
├── images/        # Uploaded avatar and campaign images
├── payment/       # Payment gateway integration (Midtrans)
├── transaction/   # Transaction domain
├── user/          # User domain
├── web/           # Admin CMS: routes, handlers, and HTML templates
│   ├── handler/     # Admin CMS handlers (user, campaign, transaction, session)
│   └── templates/    # HTML templates (layouts + per-domain views)
├── go.mod
├── go.sum
└── main.go
```

## Getting Started

1. **Clone the repository**

   ```bash
   git clone https://github.com/agusdedi/backer-backend.git
   cd backer-backend
   ```

2. **Install dependencies**

   ```bash
   go mod tidy
   ```

3. **Set up environment configuration**

   Create a `.env` file in the project root (see [Environment Variables](#environment-variables) below). These values are loaded via `godotenv` and read by `config/config.go`.

4. **Set up the database**

   Create a MySQL database matching `DB_NAME`, then run your migrations/seed data as needed.

5. **Run the application**

   ```bash
   go run main.go
   ```

   The server starts on `SERVER_PORT` (default `8080`). The REST API is available under `/api/v1`, and the Admin CMS is available at `/login`.

## Environment Variables

Create a `.env` file in the project root with the following keys:

```env
# Server
SERVER_PORT=8080
SERVER_HOST=localhost

# Database
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_NAME=backer

# Frontend / CORS
FRONTEND_URL=
EXTRA_CORS_ORIGIN=

# Images
IMAGE_BASE_URL=http://localhost:8080

# JWT (REST API auth)
JWT_SECRET_KEY=replace-with-a-long-random-string

# Session (Admin CMS auth — must differ from JWT_SECRET_KEY)
SESSION_SECRET_KEY=replace-with-a-different-long-random-string

# Midtrans (payment gateway)
MIDTRANS_SERVER_KEY=
MIDTRANS_CLIENT_KEY=
MIDTRANS_ENV=sandbox
```

| Variable | Required | Description |
|---|---|---|
| `SERVER_PORT` | No | Port the app listens on. Defaults to `8080`. |
| `SERVER_HOST` | No | Host the app binds to. Defaults to `localhost`. |
| `DB_HOST` | No | MySQL host. Defaults to `localhost`. |
| `DB_PORT` | No | MySQL port. Defaults to `3306`. |
| `DB_USER` | No | MySQL user. Defaults to `root`. |
| `DB_PASSWORD` | No | MySQL password. Leave empty if your local MySQL has none. |
| `DB_NAME` | No | Database name. Defaults to `backer`. |
| `FRONTEND_URL` | No | Frontend origin allowed by CORS, if applicable. |
| `EXTRA_CORS_ORIGIN` | No | An additional origin to allow via CORS (e.g. an ngrok URL for testing). |
| `IMAGE_BASE_URL` | No | Base URL prepended to stored image paths (avatars, campaign images). Defaults to `http://localhost:8080`. |
| `JWT_SECRET_KEY` | **Yes** | Secret used to sign REST API JWT tokens. Must not be empty. |
| `SESSION_SECRET_KEY` | **Yes** | Secret used to encrypt Admin CMS session cookies. Must not be empty, and should differ from `JWT_SECRET_KEY`. |
| `MIDTRANS_SERVER_KEY` | Only for payments | Midtrans server key for creating transactions. |
| `MIDTRANS_CLIENT_KEY` | Only for payments | Midtrans client key. |
| `MIDTRANS_ENV` | No | `sandbox` or `production`. Defaults to `sandbox`. |

> `JWT_SECRET_KEY` and `SESSION_SECRET_KEY` should each be a long, random string (e.g. generated with `openssl rand -base64 32`) and kept out of version control.

## Admin CMS

Once the server is running, visit `http://localhost:8080/login` and sign in with an account whose `role` is `admin` in the `users` table. From there you can manage users, campaigns, and transactions through the browser.

## API Documentation

Full API documentation, including all available endpoints, request/response examples, and authentication details, is published via Postman:

[![Postman Documentation](https://img.shields.io/badge/API%20Docs-Postman-FF6C37?style=for-the-badge&logo=postman&logoColor=white)](https://documenter.getpostman.com/view/30805799/2sBY4HTiWm)

Or import the collection directly into Postman:

[![Run in Postman](https://run.pstmn.io/button.svg)](https://documenter.getpostman.com/view/30805799/2sBY4HTiWm)

## Deployment

[#deployment](#deployment)

This backend is deployed as a Docker container. The full step-by-step guide covers:

- Setting up a free managed MySQL database on [Aiven](https://aiven.io)
- Configuring TLS for the database connection
- Setting up [Cloudflare R2](https://developers.cloudflare.com/r2/) for persistent file storage (campaign images & avatars)
- Deploying the backend container to [Render](https://render.com)
- Deploying the frontend to [Vercel](https://vercel.com)
- Connecting CORS between the two

See [`deployment-guide-backer.md`](./deployment-guide-backer.md) in this repository for the complete walkthrough.

## Related Repository

[#related-repository](#related-repository)

- Frontend: [github.com/agusdedi/backer-frontend](https://github.com/agusdedi/backer-frontend)

## Contributing

Contributions and suggestions are welcome. For major changes, please open an issue first to discuss what you'd like to change.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

Made by [agusdedi](https://github.com/agusdedi)