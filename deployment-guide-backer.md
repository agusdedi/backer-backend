# Backer Deployment Guide

Organized according to the established workflow, and aligned with the actual code in `backer-backend` (config.go, main.go, upload handlers) and `backer-frontend` (nuxt.config.js).

---

## 1. Set Up Aiven MySQL

**What it is:** Aiven offers a MySQL free tier that's genuinely free forever (not a trial) — 1GB storage, 1GB RAM, single node, no credit card required. Good fit for this stage. Note: the service will automatically *power off* after a long period of inactivity (you'll get an email notification first), and you can turn it back on anytime — your data isn't lost.

**Steps:**
1. Sign up at [aiven.io](https://aiven.io) — no credit card needed.
2. Create a new service → choose **MySQL** → select the **Free** plan.
3. Choose a region (pick one close to where your Render service will be, e.g. Singapore/Asia if available, to keep backend↔DB latency low).
4. Wait for the service to be running (usually 1-3 minutes).
5. On the service's **Overview** page, note down all of the following (you'll need these for your production `.env`):
   - **Host**
   - **Port**
   - **User** (default `avnadmin`)
   - **Password**
   - **Database name** (you can use the default or create a new database called `backer` via query)
6. On the **Overview** tab there's also a **CA Certificate** download button (`ca.pem`) — **you must grab this**, since Aiven requires a TLS connection. Without it, your Go backend won't be able to connect.

⚠️ Don't commit `ca.pem` to Git. This file will be uploaded directly to Render as a Secret File in step 5.

---

## 2. Add Custom TLS Code in Go

Your current code (`main.go`) connects like this:

```go
dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", ...)
db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
```

This won't work against Aiven because there's no TLS yet. Under the hood, `gorm.io/driver/mysql` uses `go-sql-driver/mysql`, and that driver requires you to **register a custom TLS config** by name, then reference that name in the DSN via `?tls=<name>`.

**a. Create a new file `config/tls.go`:**

```go
package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

// RegisterTLSConfig registers a custom TLS config named "aiven" using the
// CA certificate at DB_CA_CERT_PATH. If the env var is empty (local dev
// without TLS), it does nothing and the plain DSN is used instead.
func RegisterTLSConfig() error {
	caCertPath := os.Getenv("DB_CA_CERT_PATH")
	if caCertPath == "" {
		return nil
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA cert at %s: %w", caCertPath, err)
	}

	rootCertPool := x509.NewCertPool()
	if ok := rootCertPool.AppendCertsFromPEM(caCert); !ok {
		return fmt.Errorf("failed to append CA cert to pool")
	}

	return mysqlDriver.RegisterTLSConfig("aiven", &tls.Config{
		RootCAs: rootCertPool,
	})
}
```

**b. Update `main.go`** — call this before opening the DB connection, and append `&tls=aiven` to the DSN when TLS is active:

```go
// after config.LoadConfig()
if err := config.RegisterTLSConfig(); err != nil {
    log.Fatal("Failed to register TLS config:", err)
}

dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
    config.AppConfig.DBUser,
    config.AppConfig.DBPassword,
    config.AppConfig.DBHost,
    config.AppConfig.DBPort,
    config.AppConfig.DBName,
)
if os.Getenv("DB_CA_CERT_PATH") != "" {
    dsn += "&tls=aiven"
}

db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
```

Since `DB_CA_CERT_PATH` is empty by default (not present in your current `.env.example`), your local MySQL keeps working normally without TLS. You'll only fill this env var in later at Render (step 5), pointing to the Secret File path.

**c. Add to `.env.example`:**
```
DB_CA_CERT_PATH=
```

---

## 3. Set Up Cloudflare R2

**Why it's needed:** Render (and almost every PaaS) has an *ephemeral* filesystem — files uploaded via `c.SaveUploadedFile()` to a local folder will **disappear** every time the service restarts/redeploys. R2 becomes the permanent storage location for campaign & avatar images.

**Why R2 (instead of S3):** 10GB free tier storage, **no egress fees** (S3 charges every time a file is fetched out, R2 doesn't), and its API is S3-compatible so you can use the regular AWS SDK.

**Steps:**
1. Log in to the Cloudflare dashboard → **R2 Object Storage** menu.
   - You may be asked to add a payment method (card/PayPal) to activate R2, though it still counts toward the free tier as long as you stay under the limits.
2. Click **Create bucket** → give it a name (lowercase letters, numbers, hyphens only), e.g. `backer-uploads`. Leave the region as **Automatic**.
3. Open the bucket → **Settings** tab → enable **Public Access**:
   - Easiest way: enable the **R2.dev subdomain** (automatically gives you a public URL `https://pub-xxxx.r2.dev`).
   - If you want your own domain (cleaner, e.g. `cdn.backer.app`), you can connect a custom domain — but this is optional; the r2.dev subdomain is enough to start.
4. Note down this public URL — it will become your backend's `IMAGE_BASE_URL` value.
5. Grab your API credentials:
   - From the **R2 Overview** page, note the **Account ID** (in the right sidebar).
   - Click **Manage R2 API Tokens** → **Create User API Token**.
   - Permission: **Object Read & Write**, scoped to the `backer-uploads` bucket only (safer than all buckets).
   - Once created, note the **Access Key ID** and **Secret Access Key** — the Secret Key is **shown only once**, save it immediately.
   - S3 API endpoint: `https://<ACCOUNT_ID>.r2.cloudflarestorage.com`

Total values to note from this step: `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET_NAME`, and the bucket's public URL (for `IMAGE_BASE_URL`).

---

## 4. Replace Upload Code — Local Disk → R2 (S3 SDK)

**a. Add dependencies:**
```bash
go get github.com/aws/aws-sdk-go-v2/aws
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/credentials
go get github.com/aws/aws-sdk-go-v2/service/s3
```

**b. Create a new package `storage/r2.go`:**

```go
package storage

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	client     *s3.Client
	bucketName string
)

// Init sets up the S3 client pointed at Cloudflare R2. Call this once
// from main() after config.LoadConfig().
func Init() error {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucketName = os.Getenv("R2_BUCKET_NAME")

	if accountID == "" || accessKey == "" || secretKey == "" || bucketName == "" {
		return fmt.Errorf("R2 credentials are not fully set")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
		config.WithRegion("auto"),
	)
	if err != nil {
		return err
	}

	client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://" + accountID + ".r2.cloudflarestorage.com")
	})
	return nil
}

// UploadFile uploads a file to the configured R2 bucket at the given key
// (e.g. "images/campaign-images/1-169..-photo.png"), matching the same
// path structure the code used to save locally.
func UploadFile(ctx context.Context, file io.Reader, key, contentType string) error {
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	return err
}
```

**c. Call `storage.Init()` in `main.go`**, after `config.LoadConfig()`:
```go
if err := storage.Init(); err != nil {
    log.Fatal("Failed to init R2 storage:", err)
}
```
(add `"backer/storage"` to the imports)

**d. Replace upload calls.** The pattern is the same in 4 places: `handler/campaign.go` (~line 223), `handler/user.go` (~line 147), and the admin CMS versions in `web/handler/campaign.go` & `web/handler/user.go`.

Before:
```go
err = c.SaveUploadedFile(file, path)
if err != nil {
    // ...error handling
}
```

After:
```go
src, err := file.Open()
if err != nil {
    data := gin.H{"is_uploaded": false}
    response := helper.APIResponse(helper.MsgFailedToSaveFileToServer, http.StatusInternalServerError, "error", data)
    c.JSON(http.StatusInternalServerError, response)
    return
}
defer src.Close()

err = storage.UploadFile(c.Request.Context(), src, path, file.Header.Get("Content-Type"))
if err != nil {
    data := gin.H{"is_uploaded": false}
    response := helper.APIResponse(helper.MsgFailedToSaveFileToServer, http.StatusInternalServerError, "error", data)
    c.JSON(http.StatusInternalServerError, response)
    return
}
```
(The `path`/`fileName` built earlier — `images/campaign-images/...` and `images/avatars/...` — **doesn't need to change**, it's used directly as the R2 object key. This matters because `buildImageURL()` in `campaign/formatter.go` and `user/formatter.go` already composes the final URL as `ImageBaseURL + "/" + fileName` — once you switch `IMAGE_BASE_URL` to the public R2 URL, every image link automatically becomes correct without touching the formatter at all.)

**e. Add to `.env.example`:**
```
# Cloudflare R2 (file storage)
R2_ACCOUNT_ID=
R2_ACCESS_KEY_ID=
R2_SECRET_ACCESS_KEY=
R2_BUCKET_NAME=
```

⚠️ **Important:** `router.Static("/images", "./images")` in `main.go` can be left as-is (it won't error), but after this migration, new images will no longer go through there — all image requests will go straight to your R2 domain via `IMAGE_BASE_URL`, not your backend.

---

## 5. Deploy the Backend to Render

**Render free tier:** free Docker web service, no credit card, but it **sleeps after 15 minutes idle** and has a ~30-60 second cold start on the first request after waking. Fine for this portfolio/demo; upgrade to Starter ($7/month) later if you need always-on.

**a. Create a `Dockerfile` at the root of `backer-backend`:**
```dockerfile
# --- Build stage ---
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o backer-backend .

# --- Run stage ---
FROM alpine:3.20
RUN apk --no-cache add ca-certificates && \
    adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=builder /app/backer-backend .
COPY --from=builder /app/web/templates ./web/templates
COPY --from=builder /app/web/assets ./web/assets
RUN chown -R appuser:appuser /app
USER appuser
EXPOSE 8080
CMD ["./backer-backend"]
```
(`web/templates` and `web/assets` are copied because they're used by the admin CMS via `gin-contrib/multitemplate` and `router.Static`.)

**b. Push the Dockerfile to your repo**, then in the Render dashboard:
1. **New** → **Web Service** → connect the `backer-backend` repo.
2. Environment: choose **Docker** (Render auto-detects the `Dockerfile`).
3. Region: pick one close to your Aiven region.
4. Instance type: **Free**.

**c. Secret File for the Aiven CA certificate:**
- In the **Environment** tab → **Secret Files** → **Add Secret File**.
- Filename: `ca.pem`, content: paste the contents of `ca.pem` from Aiven.
- Render automatically mounts it to `/etc/secrets/ca.pem` at runtime.

**d. Environment Variables** — fill in all of these in the **Environment** tab:
```
SERVER_PORT=8080
SERVER_HOST=0.0.0.0

DB_DRIVER=mysql
DB_HOST=<from Aiven>
DB_PORT=<from Aiven>
DB_USER=<from Aiven>
DB_PASSWORD=<from Aiven>
DB_NAME=<database name>
DB_CA_CERT_PATH=/etc/secrets/ca.pem

IMAGE_BASE_URL=<R2 public bucket URL>
R2_ACCOUNT_ID=<from step 3>
R2_ACCESS_KEY_ID=<from step 3>
R2_SECRET_ACCESS_KEY=<from step 3>
R2_BUCKET_NAME=<from step 3>

JWT_SECRET_KEY=<generate a long random string>
SESSION_SECRET_KEY=<generate a long random string, MUST differ from JWT_SECRET_KEY>

MIDTRANS_SERVER_KEY=<your production/sandbox Midtrans key>
MIDTRANS_CLIENT_KEY=<your production/sandbox Midtrans key>
MIDTRANS_ENV=sandbox

FRONTEND_URL=          # fill in after step 6 (Vercel URL)
EXTRA_CORS_ORIGIN=      # fill in after step 6 (Vercel URL)
```
> Tip for generating a random secret key: `openssl rand -base64 32` in your terminal.

**e. Deploy.** Render will build the image from the Dockerfile and run it. Note the URL you're given, e.g. `https://backer-backend.onrender.com`.

**f. Check the logs** in the **Logs** tab — make sure you see `"Config loaded successfully"` and no DB connection errors. If you see `x509: certificate signed by unknown authority`, it means `DB_CA_CERT_PATH` is wrong or the Secret File wasn't mounted — check that the filename is exactly `ca.pem`.

---

## 6. Deploy the Frontend to Vercel

Vercel auto-detects Nuxt (zero-config) — including static output (`ssr: false`, `nitro.preset: 'static'` already set in your `nuxt.config.js`), so no build config changes are needed.

**Steps:**
1. In the Vercel dashboard → **Add New** → **Project** → import the `backer-frontend` repo.
2. Vercel auto-fills the build command & output directory — leave the defaults (usually `.output/public` for Nuxt).
3. Before deploying, open **Environment Variables**, fill in according to your `.env.example`:
```
AUTH_ORIGIN=https://<your-vercel-domain>.vercel.app
NUXT_PUBLIC_API_BASE=https://backer-backend.onrender.com/api/v1
NUXT_PUBLIC_IMAGE_BASE=https://backer-backend.onrender.com
```
   (`NUXT_PUBLIC_IMAGE_BASE` is actually no longer used for campaign/avatar images since those now come straight from the `image_url` the backend generates using the R2 `IMAGE_BASE_URL` — but fill it in anyway just in case some part of the frontend still constructs image URLs manually.)
4. Click **Deploy**. Once done, note your production domain, e.g. `https://backer.vercel.app`.
5. Go back to **Settings → Environment Variables**, update `AUTH_ORIGIN` with that final domain if it differs from your earlier guess, then **redeploy**.

---

## 7. Connect CORS

Back on Render, open your backend service → **Environment**, fill in the 2 variables left empty earlier:
```
FRONTEND_URL=https://backer.vercel.app
EXTRA_CORS_ORIGIN=https://backer.vercel.app
```
- `FRONTEND_URL` is used in `payment/service.go` for the Midtrans Snap redirect URLs (`finish`, etc.).
- `EXTRA_CORS_ORIGIN` is used in `main.go` for the CORS allowlist (`allowedOrigins`).

Save → Render will automatically redeploy the service with the new env vars. After that, you should see a line like `Allowed CORS origins: [http://localhost:3000 https://backer.vercel.app]` in the logs.

⚠️ If you're using Midtrans, don't forget to also update the **Payment Notification URL** and **Finish/Unfinish/Error Redirect URL** in the Midtrans dashboard to point to your production Render & Vercel domains, not localhost/ngrok anymore.