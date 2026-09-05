# Gocore CMS — Working Process Guide

Gocore CMS is a **headless-first CMS** built with **Go + Gin + GORM**. One backend manages every
kind of website — landing pages, blogs, personal/informational sites, e-commerce, social,
educational — while the frontend stays completely separate (Nuxt, Next, plain HTML, mobile apps).
Frontends talk to the CMS over a JSON REST API; admins manage everything from the built-in
Gocore-based admin panel.

```
┌────────────────┐        JSON over HTTP         ┌──────────────────────────┐
│  Any frontend  │  ─────────────────────────▶   │        Gocore CMS        │
│ Next/Nuxt/HTML │  ◀─────────────────────────   │  Gin + GORM + WebSocket  │
└────────────────┘        + WebSocket             └───────────┬──────────────┘
                                                              │
                                                   MySQL (default) / PostgreSQL
```

---

## 1. Requirements

- Go 1.22+
- MySQL 8 (default) **or** PostgreSQL
- Nothing else — no Node build step for the backend/admin panel.

## 2. Setup & Run

```bash
# 1. Configure
copy .env.example .env        # then edit .env

# 2. Create the database (MySQL example)
mysql -u root -p -e "CREATE DATABASE Gocore_cms CHARACTER SET utf8mb4;"

# 3. Run (tables are auto-migrated and defaults auto-seeded on boot)
go mod tidy
go run .
```

Open **http://localhost:8080** (public site) and **http://localhost:8080/login** (admin panel).

> **Default super admin:** `admin@Gocore.com` / `admin123` — created automatically the first time
> the server boots against an empty database. **Change this password immediately** from
> *My Profile* after logging in.

### Choosing the database

`.env` → `DB_DRIVER=mysql` (default) or `DB_DRIVER=postgres`. Everything else (models,
migrations, seeding) works identically on both — only the DSN changes:

| Key         | MySQL default | Postgres example |
|-------------|---------------|------------------|
| DB_DRIVER   | mysql         | postgres         |
| DB_PORT     | 3306          | 5432             |
| DB_SSLMODE  | (ignored)     | disable          |

> On this machine MySQL listens on port **3307** — that is already set in the local `.env`.

## 3. Project Structure

```
Gocore/
├── main.go                     # bootstrap: config → DB → migrate → seed → routes → listen
├── routes/routes.go            # ALL routes: public pages, auth, admin panel, /api/v1
├── internal/
│   ├── config/config.go        # .env loading, DSN builder
│   ├── database/               # connection, AutoMigrate, idempotent seeder
│   ├── models/                 # User/Role/Permission, Post/Page/Category/Tag/Media/Menu/
│   │                           # Comment, Product/Order/Transaction, ActivityLog, Setting…
│   ├── middleware/             # AuthAPI (Bearer), AuthWeb (cookie), RequirePermission,
│   │                           # ActivityLogger
│   ├── handlers/               # one file per domain (auth, user, role, content, commerce,
│   │                           # media, payment, settings, dashboard, ws, public, web)
│   ├── realtime/hub.go         # WebSocket hub (broadcast + per-user events)
│   └── utils/                  # JWT, bcrypt, JSON responses, pagination, slugs
├── templates/                  # Gocore admin templates + cms-*.html functional screens
├── static/                     # CSS/JS/images served at /assets
└── storage/uploads/            # uploaded media, served at /uploads
```

## 4. Authentication

Two session styles run side by side against the same user table:

| Client            | Mechanism                                             |
|-------------------|-------------------------------------------------------|
| Admin panel       | Login form → JWT stored in an HttpOnly cookie          |
| Headless frontend | `POST /api/v1/auth/login` → Bearer access token + rotating refresh token |

### API flow

```
POST /api/v1/auth/register   {name, email, password}
POST /api/v1/auth/login      {email, password}
   → { access_token, refresh_token, user }
GET  /api/v1/auth/me         Authorization: Bearer <access_token>
POST /api/v1/auth/refresh    {refresh_token}      # rotates the pair
POST /api/v1/auth/logout     {refresh_token}      # revokes it
PUT  /api/v1/auth/me         update own profile
PUT  /api/v1/auth/password   {current_password, new_password}
```

- Access tokens expire after `JWT_ACCESS_TTL_MIN` (default 60 min).
- Refresh tokens are stored server-side, rotate on every refresh, and are revoked on
  logout/password change.
- Self-registration can be switched off with the `allow_registration` setting; the role given
  to new sign-ups is the `default_role` setting (default: `customer`).

## 5. Roles & Permissions (RBAC)

- **Permission** = `resource.action`, e.g. `content.publish`, `users.delete`.
- **Role** = a named set of permissions. Users can hold several roles.
- `super-admin` implicitly passes every check.

Seeded roles: `super-admin`, `admin`, `editor`, `author`, `customer`. System roles cannot be
deleted; you can create unlimited custom roles from **CMS → Roles & Permissions** using the
checkbox permission matrix.

Permission groups: `users`, `roles`, `content`, `media`, `commerce`, `settings`, `activity`,
`comments`. Every admin screen and API endpoint is guarded by the matching permission — a user
without it gets a 403 (JSON on the API, an “Access Denied” page in the panel).

## 6. Activity Log

Every mutating request (POST/PUT/PATCH/DELETE) plus every login is recorded automatically with:
user, action (create/update/delete/login), entity + id, human-readable detail, IP, user agent,
method and path. Browse and filter it at **CMS → Activity Log** or via
`GET /api/v1/activity?action=&entity=&user_id=&search=`.

## 7. Admin Panel Map

Log in at `/login`. The **CMS** section in the sidebar contains the functional screens:

| Screen              | URL                        | What it does                                     |
|---------------------|----------------------------|--------------------------------------------------|
| Overview            | `/dashboard/cms`           | Live stats, recent activity, newest users        |
| Users               | `/dashboard/cms/users`     | Search, create, edit, block, delete, assign roles|
| Roles & Permissions | `/dashboard/cms/roles`     | Role list + permission matrix editor             |
| Activity Log        | `/dashboard/cms/activity`  | Filterable audit trail                           |
| Posts               | `/dashboard/cms/posts`     | Blog content: draft → publish workflow, SEO, tags|
| Categories          | `/dashboard/cms/categories`| Post & product categories                        |
| Media Library       | `/dashboard/cms/media`     | Upload files, copy public path, delete           |
| Products            | `/dashboard/cms/products`  | E-commerce catalog with price/sale/stock         |
| Orders              | `/dashboard/cms/orders`    | Order list + status workflow                     |
| Site Settings       | `/dashboard/cms/settings`  | Key/value settings grouped by area               |
| My Profile          | `/dashboard/cms/profile`   | Own name/phone + password change                 |

Everything under `/dashboard/...` (including all the Gocore demo dashboards — eCommerce, CRM,
LMS, Hospital, HRM, School, etc.) requires login.

## 8. Content API for Frontends

No auth needed — this is what your Nuxt/Next/plain-HTML site calls:

```
GET  /api/v1/public/settings                 # whitelisted site settings
GET  /api/v1/public/menus/main               # navigation menus
GET  /api/v1/public/posts?page=1&per_page=10&category=news&tag=go&search=x
GET  /api/v1/public/posts/:slug              # full post + approved comments (+1 view)
POST /api/v1/public/posts/:slug/comments     # {name,email,body} → goes to moderation
GET  /api/v1/public/pages/:slug              # static pages (About, Landing sections…)
GET  /api/v1/public/categories?type=post
GET  /api/v1/public/products?category=&search=
GET  /api/v1/public/products/:slug
POST /api/v1/public/orders                   # storefront checkout (server-side pricing)
POST /api/v1/public/contact                  # contact form
```

All list endpoints return `{success, data, meta: {total, page, per_page, last_page}}`.

Example (Next/Nuxt/fetch):

```js
const res = await fetch("http://localhost:8080/api/v1/public/posts?per_page=6");
const { data: posts } = await res.json();
```

Set `CORS_ORIGINS` in `.env` to your frontend origins (comma separated).

## 9. E-commerce & Payment Gateways

Order lifecycle: `pending → paid → processing → shipped → completed` (or
`cancelled` / `refunded`). Orders are priced **server-side** from the product table and stock is
decremented on creation.

Payments are pluggable; **Stripe** and **SSLCommerz** ship built in:

```
POST /api/v1/payments/initiate         (Bearer auth)
{ "order_id": 1, "gateway": "stripe", "success_url": "...", "cancel_url": "..." }
   → { "checkout_url": "https://checkout.stripe.com/..." }
```

Redirect the customer to `checkout_url`. The gateway calls back:

- `POST /api/v1/payments/webhook/stripe`      (checkout.session.completed → order paid)
- `POST /api/v1/payments/webhook/sslcommerz`  (IPN → order paid/failed)

Configure keys in `.env` (`STRIPE_SECRET_KEY`, `SSLCOMMERZ_STORE_ID`/`_PASSWORD`,
`SSLCOMMERZ_SANDBOX=true`). Every attempt is stored in the `transactions` table.

> Production note: verify the `Stripe-Signature` header with `STRIPE_WEBHOOK_SECRET`, and call
> the SSLCommerz validation API from the IPN handler before trusting a payment.

## 10. Realtime (WebSocket)

```js
const ws = new WebSocket("ws://localhost:8080/api/v1/ws?token=" + accessToken);
ws.onmessage = (e) => {
  const { type, payload, at } = JSON.parse(e.data);
  // types: notification, user.registered, user.created, post.created,
  //        order.created, order.updated, payment.success, contact.received
};
```

Admins can push notifications with `POST /api/v1/notifications`
(`{title, body, type, user_id?}` — omit `user_id` to broadcast). Notifications are also stored
and readable via `GET /api/v1/notifications`.

## 11. Media Uploads

- Admin panel: **CMS → Media Library** (upload, copy path, delete).
- API: `POST /api/v1/media` multipart with field `file` (Bearer auth + `media.upload`).
- Files are stored under `UPLOAD_DIR/<year>/<month>/<uuid>.<ext>` and served publicly at
  `/uploads/...`. Allowed types: images, pdf/doc/xls/csv/txt, mp3/mp4, zip. Size limit:
  `MAX_UPLOAD_MB`.

## 12. Full Admin API Reference (Bearer auth)

| Area      | Endpoints |
|-----------|-----------|
| Users     | `GET/POST /users`, `GET/PUT/DELETE /users/:id` |
| Roles     | `GET/POST /roles`, `GET/PUT/DELETE /roles/:id`, `GET /permissions` |
| Activity  | `GET /activity` |
| Posts     | `GET/POST /posts`, `GET/PUT/DELETE /posts/:id` |
| Pages     | `GET/POST /pages`, `PUT/DELETE /pages/:id` |
| Categories| `GET/POST /categories`, `PUT/DELETE /categories/:id` |
| Comments  | `GET /comments`, `PUT /comments/:id` (approve/spam), `DELETE /comments/:id` |
| Media     | `GET/POST /media`, `DELETE /media/:id` |
| Products  | `GET/POST /products`, `GET/PUT/DELETE /products/:id` |
| Orders    | `GET/POST /orders`, `GET /orders/:id`, `PUT /orders/:id/status`, `GET /transactions` |
| Payments  | `POST /payments/initiate` |
| Settings  | `GET/PUT /settings` |
| Messages  | `GET /contact-messages`, `PUT /contact-messages/:id/read`, `DELETE /contact-messages/:id` |
| Todos     | `GET/POST /todos`, `PUT /todos/:id/toggle`, `DELETE /todos/:id` |
| Notifs    | `GET/POST /notifications`, `PUT /notifications/:id/read` |
| Stats     | `GET /dashboard/stats` |

All under `/api/v1/`. Errors: `{"success": false, "error": "message"}` with proper HTTP codes
(401 unauthenticated, 403 permission denied, 404, 422 validation).

## 13. Deploying to Production

1. `APP_ENV=production` (release mode, secure cookies) and a strong random `JWT_SECRET`.
2. Build a single binary: `go build -o Gocorecms.exe .` — ship it with `templates/`, `static/`
   and your `.env`.
3. Put it behind HTTPS (nginx/Caddy) and point payment webhooks at your public URL.
4. Use a dedicated DB user (not root) with rights only on the CMS database.

## 14. Extending the CMS

Adding a new managed entity takes four steps:

1. **Model** in `internal/models/` + register it in `database.Migrate()`.
2. **Permissions** for it in `internal/database/seed.go` (`permissionCatalog`).
3. **Handler** file in `internal/handlers/` (copy an existing one — list/get/create/update/delete).
4. **Routes** in `routes/routes.go` guarded by `middleware.RequirePermission("thing.view")`,
   plus (optionally) a `templates/cms-things.html` screen and a sidebar link.

That's the whole loop — auth, RBAC, activity logging and realtime events come for free from the
middleware.
