# TDD (Technical Design Document) - TEMPLATE COMPREHENSIVE

**Project Name:** [Nama Project]  
**Author:** [Your Name]  
**Last Updated:** [Date]  
**Version:** 1.0  

---

## 📋 Table of Contents
1. [Executive Summary](#1-executive-summary)
2. [Technology Stack](#2-technology-stack)
3. [Architecture & Project Structure](#3-architecture--project-structure)
4. [Database Design](#4-database-design)
5. [API Contract](#5-api-contract)
6. [Business Logic Flow](#6-business-logic-flow)
7. [Non-Functional Requirements (NFR)](#7-non-functional-requirements)
8. [Security & Authentication](#8-security--authentication)
9. [Error Handling & Logging](#9-error-handling--logging)
10. [Testing Strategy](#10-testing-strategy)
11. [Deployment & Environment](#11-deployment--environment)
12. [Observability & Monitoring](#12-observability--monitoring)
13. [Dependencies & Versioning](#13-dependencies--versioning)
14. [Known Limitations & Future Work](#14-known-limitations--future-work)

---

## 1. Executive Summary

**Problem Being Solved:**  
[Singkat, apa masalah yang project ini solve?]

**MVP Scope:**  
[Fitur P0 yang HARUS ada untuk launch, bukan nice-to-have]

**Success Metrics:**  
- [Contoh: "User signup completion rate >80%"]
- [Contoh: "API response time <200ms p95"]
- [Contoh: "Zero unplanned downtime"]

**Out of Scope (untuk v1):**
- [Fitur yang defer ke v2+]

---

## 2. Technology Stack

### Backend
**Language & Framework:**
- **Language:** Go 1.22+
- **Framework:** Gin Gonic v1.10+
- **Rationale:** 
  - Fast, compiled language → low latency API
  - Gin is minimal & performant for REST APIs
  - Strong concurrency model (goroutines) untuk real-time features
  - Alternative considered: Node.js + Express (rejected karena overhead untuk CPU-intensive tasks)

### Frontend
**Framework & Build Tools:**
- **Framework:** Next.js 14+ (App Router)
- **UI Library:** React 18+
- **Styling:** Tailwind CSS v3+
- **Package Manager:** pnpm
- **Rationale:**
  - SSR/SSG for SEO & performance
  - Built-in routing & API routes (backend-lite use case)
  - Vercel deployment streamlined
  - Alternative: SvelteKit (rejected karena smaller ecosystem untuk production)

### Database
**Primary:**
- **Type:** PostgreSQL 15+
- **Hosting:** Neon.tech (serverless Postgres) atau AWS RDS
- **Rationale:**
  - ACID transactions required for payment/data consistency
  - JSONB support untuk semi-structured data (jika ada)
  - PostGIS jika geolocation needed
  - Neon vs RDS: Neon cheaper for low-traffic, auto-scaling, built-in backups

**Cache Layer (Optional):**
- **Type:** Redis (for session, rate-limit counters, real-time data)
- **Hosting:** Redis Cloud atau AWS ElastiCache
- **Rationale:** Reduce DB load for frequent queries (e.g., user session, leaderboard)

### External Services
| Service | Purpose | Provider | Rationale |
|---------|---------|----------|-----------|
| Email | Transactional email (welcome, reset password) | SendGrid or Resend | Reliable delivery + webhooks for bounce tracking |
| Authentication (Optional) | OAuth2 (Google, GitHub) | Firebase Auth or Auth0 | Offload security complexity |
| Storage | File uploads (avatars, documents) | AWS S3 or Cloudinary | Scalable, CDN-backed, cheaper than DB blob storage |
| Real-time (Optional) | WebSocket/Server-Sent Events | Firebase Realtime or Socket.io | Live notifications, collaborative features |

### DevOps & Deployment
- **Container:** Docker (for consistency dev→prod)
- **Container Registry:** Docker Hub or GitHub Container Registry (ghcr.io)
- **Orchestration:** 
  - **Simple:** Render.com or Railway.app (PaaS, auto-deploys)
  - **Standard:** Docker Compose locally, systemd + VPS in production
  - **Scale:** Kubernetes (defer to v2+)
- **CI/CD:** GitHub Actions
- **Rationale:** GitHub Actions free for public repos, integrates seamlessly with GitHub

---

## 3. Architecture & Project Structure

### Architecture Pattern: Clean Architecture (Hexagonal)

**Why Clean Architecture?**
- Clear separation of concerns (handler → service → repository)
- Easy to test (mock dependencies)
- Independent of framework (swap Gin for another HTTP lib easily)
- Industry standard for production systems

### Backend Folder Structure

```
project-root/
├── cmd/
│   └── api/
│       └── main.go                 # Entry point
├── internal/                       # Private to this project
│   ├── handler/
│   │   ├── auth_handler.go        # HTTP handlers for auth endpoints
│   │   ├── user_handler.go
│   │   └── health_handler.go
│   ├── service/
│   │   ├── auth_service.go        # Business logic
│   │   ├── user_service.go
│   │   └── email_service.go       # External service orchestration
│   ├── repository/
│   │   ├── user_repository.go     # Database queries
│   │   ├── auth_token_repository.go
│   │   └── query_builder.go       # Helper for complex queries
│   ├── model/
│   │   ├── user.go                # Domain models
│   │   ├── auth.go
│   │   └── error.go               # Custom error types
│   ├── middleware/
│   │   ├── auth_middleware.go     # JWT validation
│   │   ├── cors_middleware.go
│   │   ├── rate_limit_middleware.go
│   │   └── request_logger.go
│   ├── util/
│   │   ├── validator.go           # Input validation helper
│   │   ├── hash.go                # Bcrypt, hashing utils
│   │   ├── jwt.go                 # JWT creation/verification
│   │   └── error_code.go          # Standardized error codes
│   ├── config/
│   │   └── config.go              # Load env vars, config struct
│   └── database/
│       ├── postgres.go            # DB connection pooling
│       └── migration.go           # Migration runner (optional in code)
├── pkg/                           # Reusable across projects
│   ├── logger/
│   │   └── logger.go              # Structured logging
│   └── httputil/
│       └── response.go            # Standard HTTP response wrapper
├── migrations/                    # SQL migration files
│   ├── 000001_create_users_table.up.sql
│   └── 000001_create_users_table.down.sql
├── tests/
│   ├── integration/
│   │   ├── auth_test.go
│   │   └── user_test.go
│   ├── unit/
│   │   ├── service/
│   │   │   └── user_service_test.go
│   │   └── util/
│   │       └── hash_test.go
│   └── fixtures/
│       └── seed.sql               # Test data
├── scripts/
│   ├── seed-db.sh                 # Local dev setup
│   └── docker-build.sh
├── docs/
│   ├── API.md                     # Full API documentation
│   ├── ARCHITECTURE.md
│   └── DEPLOYMENT.md
├── .env.example                   # Template for env vars
├── .env.local                     # Local (git-ignored)
├── docker-compose.yml             # Local dev stack
├── Dockerfile                     # Production image
├── go.mod
├── go.sum
└── README.md

```

### Frontend Folder Structure

```
frontend/
├── app/                           # Next.js App Router
│   ├── layout.tsx                 # Root layout
│   ├── page.tsx                   # Home page
│   ├── auth/
│   │   ├── login/page.tsx
│   │   ├── register/page.tsx
│   │   └── layout.tsx             # Auth layout wrapper
│   ├── dashboard/
│   │   ├── layout.tsx
│   │   ├── page.tsx
│   │   └── [id]/page.tsx          # Dynamic route
│   └── api/
│       ├── revalidate/route.ts    # ISR revalidation endpoint
│       └── proxy/[...slug]/route.ts # (Optional) API proxy
├── components/
│   ├── common/
│   │   ├── Header.tsx
│   │   ├── Footer.tsx
│   │   └── Navbar.tsx
│   ├── auth/
│   │   ├── LoginForm.tsx
│   │   └── RegisterForm.tsx
│   ├── dashboard/
│   │   └── UserCard.tsx
│   └── ui/                        # Reusable UI (buttons, inputs, modals)
│       ├── Button.tsx
│       ├── Input.tsx
│       └── Modal.tsx
├── lib/
│   ├── api.ts                     # API client fetch wrapper
│   ├── auth.ts                    # Auth context / hooks
│   ├── hooks/
│   │   ├── useAuth.ts
│   │   └── useFetch.ts
│   └── utils.ts                   # Helper functions
├── styles/
│   ├── globals.css
│   └── variables.css              # Tailwind config overrides
├── public/
│   └── images/                    # Static assets
├── .env.local                     # Local env (API_URL, etc)
├── next.config.js
├── tailwind.config.js
├── tsconfig.json
└── package.json
```

### Data Flow Diagram

```
┌─────────────────────────────────────────────────────────┐
│ FRONTEND (Next.js)                                      │
│ ┌──────────────────────────────────────────────────┐   │
│ │ React Component → useAuth Hook → API Client      │   │
│ └──────────────────────────────────────────────────┘   │
└──────────────────┬──────────────────────────────────────┘
                   │ HTTP/REST
┌──────────────────▼──────────────────────────────────────┐
│ BACKEND (Go + Gin)                                      │
│ ┌──────────────────────────────────────────────────┐   │
│ │ HTTP Router (Gin)                                │   │
│ │   ↓ Middleware (auth, validation, logging)       │   │
│ │ Handler Layer (request parsing)                  │   │
│ │   ↓ Dependency injection (service, repo)         │   │
│ │ Service Layer (business logic, transactions)     │   │
│ │   ↓                                              │   │
│ │ Repository Layer (data access, queries)          │   │
│ └──────────────────────────────────────────────────┘   │
└──────────────────┬──────────────────────────────────────┘
                   │ SQL + Connection Pool
┌──────────────────▼──────────────────────────────────────┐
│ DATABASE (PostgreSQL)                                   │
│ Tables, Indexes, Constraints                            │
└─────────────────────────────────────────────────────────┘
```

---

## 4. Database Design

### Overview
**Total Tables:** 7 (for auth + user management MVP)  
**Primary Key Strategy:** UUID (not auto-increment integers)  
**Soft Delete:** Yes, using `deleted_at` timestamp  
**Timestamps:** All tables have `created_at` and `updated_at`  
**Concurrency Control:** Optimistic locking via `version` column (optional, for critical tables)

### Table Definitions

#### 1. `users` — User accounts

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    
    -- Profile
    avatar_url TEXT,
    bio TEXT,
    
    -- Status & Verification
    email_verified BOOLEAN DEFAULT FALSE,
    email_verified_at TIMESTAMP,
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'banned')),
    
    -- Audit
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    -- Indexing
    CONSTRAINT email_not_null CHECK (email IS NOT NULL)
);

-- Indexes (critical for login & queries)
CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_status ON users(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_created_at ON users(created_at DESC);
```

**Rationale:**
- **UUID instead of SERIAL:** Better for distributed systems, less predictable
- **password_hash:** Never store plain password; use bcrypt
- **email_verified:** Track verification state for progressive onboarding
- **status enum:** Hard-enforce valid states at DB level
- **deleted_at:** Soft delete for audit trail & recovery
- **Indexes on email, status:** Query planning optimization (email for login, status for user filtering)

---

#### 2. `auth_tokens` — JWT token tracking (optional but recommended for blacklist/revocation)

```sql
CREATE TABLE auth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    
    -- Token metadata
    token_type VARCHAR(50) DEFAULT 'access' CHECK (token_type IN ('access', 'refresh')),
    expires_at TIMESTAMP NOT NULL,
    issued_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked BOOLEAN DEFAULT FALSE,
    revoked_at TIMESTAMP,
    
    -- Audit
    ip_address INET,
    user_agent TEXT,
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Cleanup: tokens older than 30 days can be deleted
    CONSTRAINT expires_at_future CHECK (expires_at > issued_at)
);

CREATE INDEX idx_auth_tokens_user_id ON auth_tokens(user_id);
CREATE INDEX idx_auth_tokens_token_hash ON auth_tokens(token_hash);
CREATE INDEX idx_auth_tokens_expires_at ON auth_tokens(expires_at);
```

**Rationale:**
- **token_hash:** Hash token before storing (never store plaintext JWT in DB)
- **token_type:** Support multiple token types (access, refresh)
- **revoked:** For logout / forced expiration
- **ip_address, user_agent:** Security audit trail (detect unusual login patterns)

---

#### 3. `email_verification_codes` — One-time codes for email verification

```sql
CREATE TABLE email_verification_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code VARCHAR(6) NOT NULL,
    
    used BOOLEAN DEFAULT FALSE,
    used_at TIMESTAMP,
    expires_at TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP + INTERVAL '24 hours'),
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT code_format CHECK (code ~ '^\d{6}$')
);

CREATE INDEX idx_email_verification_codes_user_id ON email_verification_codes(user_id);
CREATE INDEX idx_email_verification_codes_expires_at ON email_verification_codes(expires_at);

-- Cleanup job: DELETE FROM email_verification_codes WHERE expires_at < NOW() AND NOT used;
```

**Rationale:**
- **6-digit code:** Balance security vs usability
- **Expiration:** 24 hours standard
- **Cleanup:** Add scheduled job to remove expired codes

---

#### 4. `password_reset_tokens` — Password reset flow

```sql
CREATE TABLE password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    
    used BOOLEAN DEFAULT FALSE,
    used_at TIMESTAMP,
    expires_at TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP + INTERVAL '1 hour'),
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT single_active_per_user 
        CHECK (NOT (used = FALSE AND expires_at > CURRENT_TIMESTAMP))
);

CREATE INDEX idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX idx_password_reset_tokens_token_hash ON password_reset_tokens(token_hash);
```

**Rationale:**
- **1-hour expiration:** Balance security vs UX (user may not check email immediately)
- **token_hash:** Same hashing strategy as auth_tokens

---

#### 5. `audit_logs` — Compliance & security audit trail

```sql
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- Event
    action VARCHAR(100) NOT NULL,  -- 'user.created', 'user.login', 'user.password_changed', etc
    resource_type VARCHAR(100),    -- 'user', 'token', etc
    resource_id UUID,
    
    -- Details
    changes JSONB,                 -- Old value → new value for sensitive fields
    ip_address INET,
    user_agent TEXT,
    
    -- Status
    status VARCHAR(50) DEFAULT 'success' CHECK (status IN ('success', 'failure')),
    error_message TEXT,
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Immutable
    CONSTRAINT immutable_audit CHECK (id IS NOT NULL)
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
```

**Rationale:**
- **JSONB for changes:** Flexible schema for different resource types
- **Immutable:** Audit logs should never be updated
- **Index on action:** Quick filtering for "all failed logins" or "all password resets"

---

#### 6. `rate_limit_counters` — For rate limiting (in-memory via Redis preferred, but DB backup)

```sql
CREATE TABLE rate_limit_counters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(255) NOT NULL UNIQUE,  -- 'login:user@email.com', 'api:192.168.1.1', etc
    
    count INT DEFAULT 0,
    reset_at TIMESTAMP NOT NULL,
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_rate_limit_counters_reset_at ON rate_limit_counters(reset_at);

-- Cleanup: DELETE FROM rate_limit_counters WHERE reset_at < NOW();
```

**Rationale:**
- **Backup strategy:** Primary rate limiting via Redis (fast), DB as fallback
- **Reset window:** Automatic cleanup via index

---

#### 7. `feature_flags` — Feature toggles (optional but production-ready)

```sql
CREATE TABLE feature_flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    
    enabled BOOLEAN DEFAULT FALSE,
    percentage_rollout INT DEFAULT 0 CHECK (percentage_rollout >= 0 AND percentage_rollout <= 100),
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Examples:
-- INSERT INTO feature_flags (name, description, enabled, percentage_rollout) VALUES
-- ('new_dashboard_ui', 'Rollout new dashboard design', FALSE, 10),  -- 10% users
-- ('dark_mode', 'Dark mode support', TRUE, 100);                    -- 100% users
```

**Rationale:**
- **Percentage rollout:** Gradual rollout without redeploying
- **Useful for:** A/B testing, gradual feature releases

---

### Migration Strategy

Use **golang-migrate** for managing schema changes:

```bash
# Generate migration
migrate create -ext sql -dir migrations -seq create_users_table

# Apply migrations (auto-run on startup or manual)
migrate -path ./migrations -database "postgres://..." up

# Rollback
migrate -path ./migrations -database "postgres://..." down 1
```

**Migration Files Example:**

```sql
-- migrations/000001_create_users_table.up.sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    ...
);

-- migrations/000001_create_users_table.down.sql
DROP TABLE users;
```

**Best Practices:**
- One logical change per migration file
- Always include both `.up.sql` and `.down.sql`
- Test rollback before deployment
- Never use `DROP TABLE` lightly (add `IF EXISTS` for safety)

---

## 5. API Contract

### API Overview

**Base URL:** `https://api.example.com/v1`  
**Response Format:** JSON  
**Authentication:** JWT Bearer token in `Authorization` header  
**Content-Type:** `application/json`

### Standard Response Wrapper

```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Error Response:**
```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "INVALID_EMAIL",
    "message": "Email format is invalid",
    "details": { "field": "email" }
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

---

### P0 Endpoints (MVP - Must Have)

#### 1. **POST /auth/register** — Create user account

**Purpose:** User registration with email & password

**Request:**
```json
{
  "email": "user@example.com",
  "name": "John Doe",
  "password": "SecurePass123!"
}
```

**Validation Rules:**
- `email`: Valid email format, not already registered
- `name`: 2-50 characters, no special chars
- `password`: Min 8 chars, 1 uppercase, 1 number, 1 special char

**Success Response (201 Created):**
```json
{
  "success": true,
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_in": 3600
  },
  "error": null,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Error Responses:**
- `400 Bad Request` — Validation failed (email, password format)
- `409 Conflict` — Email already registered
- `500 Internal Server Error` — Database error

**Flow:**
1. Validate input (format, length)
2. Check email uniqueness (query DB)
3. Hash password with bcrypt
4. BEGIN TRANSACTION
   - INSERT INTO users (email, name, password_hash)
   - INSERT INTO email_verification_codes (code)
5. COMMIT
6. Send verification email (async)
7. Return 201 + token

---

#### 2. **POST /auth/login** — Authenticate & get token

**Purpose:** User login with email & password

**Request:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!"
}
```

**Success Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "name": "John Doe",
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_in": 3600
  },
  "error": null,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Error Responses:**
- `400 Bad Request` — Missing email or password
- `401 Unauthorized` — Invalid credentials (no details to prevent user enumeration)
- `429 Too Many Requests` — Rate limited (>5 failed attempts in 15 min)
- `500 Internal Server Error` — Server error

**Flow:**
1. Validate input (email, password not empty)
2. Query user by email (if not found → 401)
3. Verify password hash with bcrypt (bcrypt.CompareHashAndPassword)
4. Check user status (active, not banned)
5. Generate JWT token (exp: 1 hour)
6. (Optional) Generate refresh token (exp: 30 days)
7. INSERT INTO auth_tokens (for tracking & revocation)
8. INSERT INTO audit_logs (login event)
9. Return 200 + token

---

#### 3. **GET /users/profile** — Get current user profile

**Purpose:** Fetch authenticated user's profile

**Authentication:** Required (Bearer token in header)

**Success Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "name": "John Doe",
    "avatar_url": "https://cdn.example.com/avatars/123.jpg",
    "bio": "Software engineer",
    "email_verified": true,
    "status": "active",
    "created_at": "2024-01-10T08:00:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  },
  "error": null,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Error Responses:**
- `401 Unauthorized` — Missing or invalid token
- `404 Not Found` — User not found (should not happen if token is valid)
- `500 Internal Server Error` — Server error

**Flow:**
1. Extract & validate JWT token from Authorization header
2. Query user by ID from token claims
3. Check user status (not soft-deleted)
4. Return 200 + user data (exclude password_hash)

---

#### 4. **PUT /users/profile** — Update user profile

**Purpose:** Update authenticated user's profile

**Authentication:** Required

**Request:**
```json
{
  "name": "John Doe Updated",
  "bio": "Senior software engineer",
  "avatar_url": "https://cdn.example.com/avatars/456.jpg"
}
```

**Validation Rules:**
- `name`: 2-50 characters (optional, if not provided, keep existing)
- `bio`: Max 500 characters
- `avatar_url`: Valid URL format

**Success Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "John Doe Updated",
    "bio": "Senior software engineer",
    "avatar_url": "https://cdn.example.com/avatars/456.jpg",
    "updated_at": "2024-01-15T10:35:00Z"
  },
  "error": null,
  "timestamp": "2024-01-15T10:35:00Z"
}
```

**Error Responses:**
- `400 Bad Request` — Validation failed
- `401 Unauthorized` — Invalid token
- `500 Internal Server Error` — Server error

**Flow:**
1. Authenticate user
2. Validate input
3. UPDATE users SET name=?, bio=?, avatar_url=?, updated_at=NOW()
4. INSERT INTO audit_logs (profile update event)
5. Return 200 + updated user data

---

#### 5. **POST /auth/verify-email** — Verify email with OTP code

**Purpose:** Verify email using code sent to inbox

**Request:**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "code": "123456"
}
```

**Success Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "email_verified": true,
    "email_verified_at": "2024-01-15T10:40:00Z"
  },
  "error": null,
  "timestamp": "2024-01-15T10:40:00Z"
}
```

**Error Responses:**
- `400 Bad Request` — Invalid code format
- `401 Unauthorized` — Code expired or incorrect
- `404 Not Found` — User not found
- `500 Internal Server Error` — Server error

**Flow:**
1. Validate user_id & code format
2. Query email_verification_codes by (user_id, code)
3. Check: not expired, not already used
4. If invalid → 401
5. UPDATE users SET email_verified=TRUE, email_verified_at=NOW()
6. UPDATE email_verification_codes SET used=TRUE, used_at=NOW()
7. INSERT INTO audit_logs (email verified event)
8. Return 200

---

#### 6. **POST /auth/logout** — Logout (revoke token)

**Purpose:** Revoke current token

**Authentication:** Required

**Success Response (200 OK):**
```json
{
  "success": true,
  "data": null,
  "error": null,
  "timestamp": "2024-01-15T10:45:00Z"
}
```

**Error Responses:**
- `401 Unauthorized` — Invalid token
- `500 Internal Server Error` — Server error

**Flow:**
1. Extract token hash
2. UPDATE auth_tokens SET revoked=TRUE, revoked_at=NOW() WHERE token_hash=?
3. INSERT INTO audit_logs (logout event)
4. Return 200

---

### HTTP Status Codes Reference

| Code | Scenario |
|------|----------|
| **200 OK** | Successful read/update |
| **201 Created** | Resource created (POST /register) |
| **400 Bad Request** | Validation error (email format, required fields) |
| **401 Unauthorized** | Invalid/missing auth token |
| **403 Forbidden** | Authenticated but not authorized (e.g., user trying to delete another user) |
| **404 Not Found** | Resource doesn't exist |
| **409 Conflict** | Resource already exists (email duplicate) |
| **429 Too Many Requests** | Rate limit exceeded |
| **500 Internal Server Error** | Unexpected server error |
| **503 Service Unavailable** | Database/external service down |

---

### Rate Limiting

**Strategy:** Token bucket algorithm

**Rules:**
- **Login endpoint:** 5 requests per 15 minutes per IP
- **Register endpoint:** 10 requests per hour per IP
- **General API:** 100 requests per 15 minutes per user
- **Public endpoints:** 1000 requests per hour per IP

**Response Header (429):**
```
Retry-After: 60
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1705318800
```

---

### Pagination (for list endpoints, added later)

**Query Parameters:**
```
GET /users?page=1&limit=20&sort=created_at&order=desc
```

**Response:**
```json
{
  "success": true,
  "data": {
    "items": [...],
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 150,
      "total_pages": 8
    }
  },
  "error": null,
  "timestamp": "..."
}
```

---

## 6. Business Logic Flow

### Feature 1: User Registration (P0 - Blocking MVP)

**Precondition:** User accesses registration form

**Main Flow:**

```
1. Frontend: User fills form (email, name, password, confirm password)
   └─> Validate locally (format checks)

2. Frontend: Submit POST /auth/register
   └─> Send {email, name, password} to backend

3. Backend Handler: Parse request
   └─> Validate input format, length, character rules
       Error path → return 400 + validation error details

4. Backend Service: Check business rules
   └─> Query: SELECT id FROM users WHERE email = ? AND deleted_at IS NULL
       └─> If exists → return 409 Conflict (email taken)
   └─> If not exists → proceed

5. Backend Service: Hash password
   └─> bcrypt.GenerateFromPassword(password, cost=12)
       └─> Blocking operation (~100ms), run in goroutine or accept latency

6. Backend: Start database transaction
   └─> BEGIN TRANSACTION

7. Backend: Insert user record
   └─> INSERT INTO users (email, name, password_hash, status, created_at)
       VALUES (?, ?, ?, 'active', NOW())
       └─> RETURNING id
       Error path → ROLLBACK → return 500

8. Backend: Generate verification code
   └─> code = random 6-digit number
   └─> INSERT INTO email_verification_codes 
       (user_id, code, expires_at)
       VALUES (user_id, code, NOW() + 24 hours)
       Error path → ROLLBACK → return 500

9. Backend: Commit transaction
   └─> COMMIT TRANSACTION
   
10. Backend: Audit logging (async)
    └─> INSERT INTO audit_logs 
        (user_id, action, status, ip_address, user_agent)
        VALUES (user_id, 'user.registered', 'success', ip, ua)

11. Backend: Send verification email (async, non-blocking)
    └─> Queue: {user_id, email, code}
    └─> Email service: Send email with code
        └─> If fails: log error, NOT a blocker (user can resend code)

12. Backend: Generate JWT token
    └─> payload: {user_id, email, exp: now + 1 hour, iat: now}
    └─> Signature: HMAC-SHA256(payload, JWT_SECRET)
    └─> Store token hash in auth_tokens table
    
13. Backend: Return 201 Created
    └─> Response: {user_id, email, access_token, expires_in}
    └─> Frontend: Redirect to email verification page OR dashboard

14. Frontend: Show "Check your email" message
    └─> Prompt user to enter 6-digit code
```

**Error Paths:**

```
❌ Email already exists:
   → Return 409 Conflict
   → Message: "Email already registered"
   → Suggestion: "Try login instead"

❌ Weak password:
   → Return 400 Bad Request
   → Message: "Password must have uppercase, lowercase, number, special char"

❌ Invalid email format:
   → Return 400 Bad Request
   → Message: "Invalid email address"

❌ Database error (constraint violation, timeout):
   → ROLLBACK transaction
   → Return 500 Internal Server Error
   → Log: stack trace, request context
   → Alert: PagerDuty/Slack

❌ Email service down:
   → Still return 201 (registration successful)
   → Log: "Email send failed for user_id X"
   → Queue for retry (5 attempts, exponential backoff)

❌ JWT generation fails:
   → Unlikely, but return 500
   → Log: "JWT generation failed"
```

**Non-Happy Path Example (User tries duplicate email):**

```
User A registers with "john@example.com" at 10:00 AM
User A confirms email

User B tries register with "john@example.com" at 10:05 AM
├─ Backend validates "john@example.com"
├─ Query: SELECT id FROM users WHERE email = 'john@example.com' AND deleted_at IS NULL
├─ Result: Found (User A's row)
└─ Return 409 Conflict
   {
     "success": false,
     "error": {
       "code": "EMAIL_ALREADY_EXISTS",
       "message": "Email already registered",
       "details": { "suggestion": "Try login or reset password" }
     }
   }
```

---

### Feature 2: User Login (P0 - Blocking MVP)

**Precondition:** User has registered & verified email

**Main Flow:**

```
1. Frontend: User fills login form (email, password)
   └─> Validate locally (format checks)

2. Frontend: Submit POST /auth/login
   └─> Send {email, password} to backend

3. Backend Handler: Validate input
   └─> Check email not empty, password not empty
       Error → return 400

4. Backend: Rate limiting check
   └─> Query: SELECT COUNT(*) FROM rate_limit_counters 
       WHERE key = 'login:192.168.1.1' AND reset_at > NOW()
   └─> If count >= 5 → return 429 Too Many Requests
   
5. Backend: Query user by email
   └─> SELECT id, email, password_hash, status FROM users 
       WHERE email = ? AND deleted_at IS NULL
   └─> If not found → return 401 (don't reveal if email exists)

6. Backend: Verify password
   └─> bcrypt.CompareHashAndPassword(stored_hash, submitted_password)
   └─> If NOT match → Increment rate limit counter → return 401
   └─> On too many failures (5+), optionally lock account temporarily

7. Backend: Check user status
   └─> If status = 'banned' → return 401 (account banned)
   └─> If status = 'inactive' → return 403 (account suspended)

8. Backend: Start transaction
   └─> BEGIN TRANSACTION

9. Backend: Generate tokens
   └─> Access token: JWT(exp: 1 hour)
   └─> Refresh token: JWT(exp: 30 days)
   └─> token_hash = SHA256(access_token)

10. Backend: Store token
    └─> INSERT INTO auth_tokens 
        (user_id, token_hash, token_type, expires_at, ip_address, user_agent)
        VALUES (user_id, hash, 'access', NOW() + 1 hour, ip, ua)

11. Backend: Clear rate limit counter (successful login)
    └─> DELETE FROM rate_limit_counters 
        WHERE key = 'login:192.168.1.1'

12. Backend: Audit log (async)
    └─> INSERT INTO audit_logs
        (user_id, action, status, ip_address)
        VALUES (user_id, 'user.login', 'success', ip)

13. Backend: COMMIT

14. Backend: Return 200 OK
    └─> Response: {user_id, email, name, access_token, refresh_token, expires_in}

15. Frontend: Store token (localStorage or sessionStorage)
    └─> Set Authorization header for future requests
    └─> Redirect to /dashboard

16. Frontend: (Optional) Refresh token lifecycle
    └─> On token close to expiry (last 5 min), auto-refresh using refresh token
    └─> POST /auth/refresh with {refresh_token}
```

**Error Paths:**

```
❌ Email not found:
   → Return 401 Unauthorized
   → Message: "Invalid email or password"
   → Log: attempt but DON'T log email (privacy)

❌ Password incorrect:
   → Return 401 Unauthorized
   → Message: "Invalid email or password"
   → Increment rate limit counter
   → Log: failed attempt

❌ Account banned:
   → Return 401 Unauthorized
   → Message: "Account has been suspended"
   → Log: attempt to access banned account

❌ Rate limited:
   → Return 429 Too Many Requests
   → Header: Retry-After: 900 (15 minutes in seconds)
   → Message: "Too many login attempts. Try again in 15 minutes"

❌ Database error:
   → ROLLBACK transaction
   → Return 500 Internal Server Error
   → Alert: Log to monitoring system
```

---

### Feature 3: Email Verification (P0 - Required for full account access)

**Precondition:** User registered, received email with code

**Main Flow:**

```
1. Frontend: User navigates to /verify-email
   └─> Shows form: "Enter 6-digit code"

2. Frontend: User receives email
   └─> Subject: "Verify your email address"
   └─> Body: "Your verification code: 123456"
   └─> Expires in 24 hours

3. Frontend: User submits POST /auth/verify-email
   └─> Send {user_id, code}

4. Backend: Validate input
   └─> Check code is 6 digits
       Error → return 400

5. Backend: Query verification code
   └─> SELECT id, used, expires_at FROM email_verification_codes
       WHERE user_id = ? AND code = ? 
       LIMIT 1
   └─> If not found → return 401

6. Backend: Validate code state
   └─> Check: NOT used
   └─> Check: NOT expired (expires_at > NOW())
   └─> If either fails → return 401 (incorrect or expired)

7. Backend: Start transaction
   └─> BEGIN TRANSACTION

8. Backend: Update user
   └─> UPDATE users 
       SET email_verified = TRUE, 
           email_verified_at = NOW()
       WHERE id = user_id

9. Backend: Mark code as used
   └─> UPDATE email_verification_codes
       SET used = TRUE, used_at = NOW()
       WHERE id = code_id

10. Backend: Audit log (async)
    └─> INSERT INTO audit_logs
        (user_id, action, status)
        VALUES (user_id, 'user.email_verified', 'success')

11. Backend: COMMIT

12. Backend: Return 200 OK
    └─> Response: {email_verified: true}

13. Frontend: Show success message
    └─> "Email verified successfully!"
    └─> Redirect to /dashboard
```

**Error Paths:**

```
❌ Code not found for user:
   → Return 401 Unauthorized
   → Message: "Invalid code"
   → Suggestion: "Check your email or request a new code"

❌ Code expired (>24 hours old):
   → Return 401 Unauthorized
   → Message: "Code expired. Request a new one"
   → Offer resend button → POST /auth/resend-verification-code

❌ Code already used:
   → Return 401 Unauthorized
   → Message: "This code was already used"

❌ Too many failed attempts:
   → Return 429 Too Many Requests
   → Lock code for 15 minutes or require new code

❌ Database error:
   → ROLLBACK
   → Return 500 Internal Server Error
```

---

### Feature 4: Password Reset (P1 - Important but not blocking MVP)

**Flow Summary:**

```
User clicks "Forgot password"
    ↓
1. POST /auth/forgot-password {email}
   - Query user by email
   - Generate reset token (UUID)
   - INSERT into password_reset_tokens (expires in 1 hour)
   - Send email with reset link
   - Return 200 (success, but don't reveal if email exists)

2. User receives email
   - Link: https://app.example.com/reset-password?token=xyz

3. Frontend: GET /auth/verify-reset-token?token=xyz
   - Validate token exists & not expired & not used
   - Return 200 if valid (enables submit button)
   - Return 401 if invalid/expired

4. User fills new password, submits
   - POST /auth/reset-password {token, new_password}
   - Validate token & new password
   - UPDATE users SET password_hash = ? WHERE id = (from token)
   - Mark token as used
   - INSERT audit_log
   - Return 200

5. Frontend: Redirect to login
   - User logs in with new password
```

---

## 7. Non-Functional Requirements (NFR)

### Performance

| Metric | Target | Rationale |
|--------|--------|-----------|
| **API Response Time (p95)** | <200ms | User perception: snappy, not sluggish |
| **API Response Time (p99)** | <500ms | Acceptable worst-case |
| **Database Query (p95)** | <50ms | Most queries should hit indexed columns |
| **Concurrent Users** | 100+ | Small MVP doesn't need millions |
| **Page Load Time (FCP)** | <1.5s | Vercel Next.js with optimized images |
| **TTFB (Time to First Byte)** | <200ms | Server response + network |

**Measurement:**
```bash
# Backend: Add to middleware
func loggingMiddleware(c *gin.Context) {
    start := time.Now()
    c.Next()
    duration := time.Since(start).Milliseconds()
    log.Infof("endpoint=%s method=%s duration_ms=%d status=%d", 
        c.Request.URL.Path, c.Request.Method, duration, c.Writer.Status())
}

# Monitor: Grafana dashboard with latency histograms
```

---

### Availability & Reliability

| Metric | Target | Rationale |
|--------|--------|-----------|
| **Uptime (SLA)** | 99.5% (~3.7 hours downtime/month) | Acceptable for MVP |
| **MTTR (Mean Time To Recovery)** | <15 minutes | Quick detection + auto-restart |
| **Database Backup Frequency** | Every 6 hours | Daily is standard, 6h is acceptable |
| **RTO (Recovery Time Objective)** | <30 minutes | How fast to restore from backup |
| **RPO (Recovery Point Objective)** | <1 hour | Max data loss: 1 hour of transactions |

**Implementation:**
- Docker auto-restart: `restart_policy: unless-stopped`
- Health check endpoint: `GET /health` (returns 200 if DB connected)
- Database: Automated backups (daily snapshots on Neon.tech)
- Monitoring: Alert if downtime detected

---

### Scalability

| Aspect | Initial | Scaling Path |
|--------|---------|--------------|
| **Database Connections** | 10 | Increase pool size (max 30 for serverless) |
| **Cache Layer** | None (optional) | Add Redis for session/rate-limit counters if CPU bottleneck |
| **Deployment** | Single instance (VPS) | Horizontal: multiple instances + load balancer (v2+) |
| **Traffic** | 100 req/s | Current: VPS handles OK. Beyond: add caching layer |

---

### Security

See [Section 8: Security & Authentication](#8-security--authentication) for detailed requirements.

---

### Maintainability

| Aspect | Target |
|--------|--------|
| **Code Coverage** | >80% for service layer, >70% overall |
| **Documentation** | README + API docs (Swagger optional) |
| **Deployment Automation** | 1-click deploy (GitHub Actions) |
| **Logging** | Structured logs (JSON), searchable in ELK or CloudWatch |

---

## 8. Security & Authentication

### Authentication Strategy: JWT (Stateless)

**Why JWT?**
- Stateless: No server-side session storage
- Scalable: Each request is self-contained
- Mobile-friendly: Works with APIs

**Token Structure:**

```json
// Header
{
  "alg": "HS256",
  "typ": "JWT",
  "kid": "key_id_2024_01"  // Key ID for rotation
}

// Payload
{
  "sub": "550e8400-e29b-41d4-a716-446655440000",  // user_id
  "email": "user@example.com",
  "roles": ["user"],
  "iat": 1705328400,      // issued at
  "exp": 1705332000,      // expires in 1 hour
  "iss": "api.example.com",
  "aud": "example.com"
}

// Signature
HMACSHA256(base64UrlEncode(header) + "." + base64UrlEncode(payload), secret)
```

**Token Generation (Go):**

```go
import "github.com/golang-jwt/jwt/v5"

func GenerateToken(userID, email string) (string, error) {
    claims := jwt.MapClaims{
        "sub":   userID,
        "email": email,
        "roles": []string{"user"},
        "iat":   time.Now().Unix(),
        "exp":   time.Now().Add(time.Hour).Unix(),
        "iss":   "api.example.com",
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}
```

**Token Storage (Frontend):**

```javascript
// localStorage (NOT recommended for sensitive data)
localStorage.setItem('token', accessToken);

// OR sessionStorage (better, cleared on browser close)
sessionStorage.setItem('token', accessToken);

// OR httpOnly cookie (BEST, immune to XSS)
document.cookie = `token=${accessToken}; HttpOnly; Secure; SameSite=Strict; Path=/;`;
```

**Token Validation (Go Middleware):**

```go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract token from header
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(401, gin.H{"error": "missing authorization header"})
            c.Abort()
            return
        }
        
        // Parse "Bearer <token>"
        const bearerScheme = "Bearer "
        if !strings.HasPrefix(authHeader, bearerScheme) {
            c.JSON(401, gin.H{"error": "invalid authorization scheme"})
            c.Abort()
            return
        }
        
        tokenString := authHeader[len(bearerScheme):]
        
        // Verify token
        token, err := jwt.ParseWithClaims(tokenString, &jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
            }
            return []byte(os.Getenv("JWT_SECRET")), nil
        })
        
        if err != nil || !token.Valid {
            c.JSON(401, gin.H{"error": "invalid token"})
            c.Abort()
            return
        }
        
        // Extract claims
        claims := token.Claims.(*jwt.MapClaims)
        userID := (*claims)["sub"].(string)
        
        // Attach to context
        c.Set("user_id", userID)
        c.Set("claims", claims)
        
        c.Next()
    }
}
```

---

### Authorization: Role-Based Access Control (RBAC)

**User Roles (v1):**
- `user` — Regular user (default)
- `admin` — Administrator (manage users, view logs)

**Roles in Token:**
```json
{
  "sub": "user_id",
  "roles": ["user"],
  "permissions": ["read:profile", "write:profile"]
}
```

**Authorization Middleware:**

```go
func RequireRole(requiredRole string) gin.HandlerFunc {
    return func(c *gin.Context) {
        claims := c.MustGet("claims").(*jwt.MapClaims)
        roles := (*claims)["roles"].([]interface{})
        
        hasRole := false
        for _, role := range roles {
            if role.(string) == requiredRole {
                hasRole = true
                break
            }
        }
        
        if !hasRole {
            c.JSON(403, gin.H{"error": "insufficient permissions"})
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// Usage in routes
r.DELETE("/users/:id", AuthMiddleware(), RequireRole("admin"), DeleteUserHandler)
```

---

### Password Security

**Hashing Strategy: Bcrypt**

```go
import "golang.org/x/crypto/bcrypt"

// Hash password on registration
const bcryptCost = 12  // Balance: security vs latency (~100ms)

func HashPassword(password string) (string, error) {
    return bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
}

// Verify on login
func VerifyPassword(hashedPassword, plainPassword string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
    return err == nil
}
```

**Why Bcrypt?**
- Adaptive: Cost factor can increase with hardware improvements
- Salted: Prevents rainbow table attacks
- Slow: ~100ms per hash, making brute force infeasible

**Password Policy (Enforce):**
- Minimum 8 characters
- At least 1 uppercase letter
- At least 1 lowercase letter
- At least 1 number
- At least 1 special character (!@#$%^&*)

**Implementation:**
```go
import "github.com/go-playground/validator/v10"

func validatePassword(password string) error {
    if len(password) < 8 {
        return fmt.Errorf("password too short")
    }
    if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
        return fmt.Errorf("password missing uppercase")
    }
    if !regexp.MustCompile(`[a-z]`).MatchString(password) {
        return fmt.Errorf("password missing lowercase")
    }
    if !regexp.MustCompile(`[0-9]`).MatchString(password) {
        return fmt.Errorf("password missing number")
    }
    if !regexp.MustCompile(`[!@#$%^&*]`).MatchString(password) {
        return fmt.Errorf("password missing special character")
    }
    return nil
}
```

---

### Rate Limiting

**Strategy: Token Bucket Algorithm**

**Endpoints & Rules:**

```go
// Separate limits per endpoint
var (
    loginLimit      = rate.NewLimiter(rate.Limit(5.0/60), 5)        // 5 per minute
    registerLimit   = rate.NewLimiter(rate.Limit(10.0/3600), 10)    // 10 per hour
    generalApiLimit = rate.NewLimiter(rate.Limit(100.0/60), 100)    // 100 per minute
)

func RateLimitMiddleware(limiter *rate.Limiter) gin.HandlerFunc {
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.JSON(429, gin.H{
                "error": "rate limit exceeded",
                "retry_after": 60,
            })
            c.Header("Retry-After", "60")
            c.Abort()
            return
        }
        c.Next()
    }
}

// Usage
r.POST("/auth/login", RateLimitMiddleware(loginLimit), LoginHandler)
r.POST("/auth/register", RateLimitMiddleware(registerLimit), RegisterHandler)
```

**Alternative: Redis-backed for distributed systems (v2+)**

```go
import "github.com/ulule/limiter/v3"

limiter := limiter.New(
    store.NewRedisStore(redisClient),
    rate.NewFromFormatted("100-M"),  // 100 per minute
)
```

---

### Input Validation

**Server-side validation (required, never trust client):**

```go
import "github.com/go-playground/validator/v10"

type RegisterRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Name     string `json:"name" validate:"required,min=2,max=50"`
    Password string `json:"password" validate:"required,min=8"`
}

func ValidateRegisterRequest(req RegisterRequest) error {
    validate := validator.New()
    return validate.Struct(req)
}

// Usage in handler
var req RegisterRequest
if err := c.BindJSON(&req); err != nil {
    c.JSON(400, gin.H{"error": "invalid json"})
    return
}

if err := ValidateRegisterRequest(req); err != nil {
    c.JSON(400, gin.H{"error": err.Error()})
    return
}
```

---

### Injection Attack Prevention

**SQL Injection Prevention: Parameterized Queries**

```go
// ❌ WRONG - SQL Injection vulnerability
query := fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", email)
rows, _ := db.Query(query)

// ✅ CORRECT - Parameterized query
query := "SELECT * FROM users WHERE email = $1"
row := db.QueryRow(query, email)
```

**XSS Prevention: Output Escaping**

```javascript
// ❌ WRONG - XSS vulnerability
element.innerHTML = userData.bio;

// ✅ CORRECT - Set as text content
element.textContent = userData.bio;

// OR use framework (React auto-escapes)
<div>{userData.bio}</div>
```

---

### CORS (Cross-Origin Resource Sharing)

**Configuration:**

```go
import "github.com/gin-contrib/cors"

config := cors.DefaultConfig()
config.AllowOrigins = []string{"https://example.com", "https://app.example.com"}
config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
config.AllowHeaders = []string{"Authorization", "Content-Type"}
config.ExposeHeaders = []string{"X-Total-Count"}  // Custom headers to expose
config.AllowCredentials = true
config.MaxAge = 12 * time.Hour

r.Use(cors.New(config))
```

---

### HTTPS & TLS

**Requirement:** All traffic must be encrypted in production

```bash
# Using Let's Encrypt (free, auto-renewal)
# Option 1: Certbot (manual)
certbot certonly --standalone -d api.example.com

# Option 2: Caddy (auto-renews)
# Place Caddyfile in VPS
api.example.com {
    reverse_proxy localhost:8080
}
```

**Redirect HTTP → HTTPS:**

```go
func redirectHTTP() {
    log.Fatal(http.ListenAndServe(":80", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
    })))
}

go redirectHTTP()

// Listen on HTTPS
log.Fatal(http.ListenAndServeTLS(":443", "cert.pem", "key.pem", handler))
```

---

### Secret Management

**Environment Variables (for local dev & single VPS):**

```bash
# .env (git-ignored)
DB_URL=postgres://user:pass@localhost/dbname
JWT_SECRET=your_secret_key_min_32_chars
SENDGRID_API_KEY=SG.xxxxx
AWS_S3_BUCKET=my-bucket
API_PORT=8080
```

**Loading in Code:**

```go
import "github.com/joho/godotenv"

func init() {
    godotenv.Load()  // Load from .env
}

// Or use environment variables directly
dbUrl := os.Getenv("DB_URL")
jwtSecret := os.Getenv("JWT_SECRET")
```

**Production Secret Management (better for larger systems):**
- **Render/Railway/Vercel:** Built-in secret management (paste secrets in dashboard)
- **AWS Secrets Manager:** For AWS VPS
- **HashiCorp Vault:** For enterprise systems (overkill for MVP)

---

### Sensitive Data Logging

**Rules:**
- ❌ Never log passwords, tokens, credit card numbers
- ❌ Never log PII (Personally Identifiable Information) unless necessary
- ✅ Log action, user_id, timestamp, result, error code
- ✅ Log IP address for security audit

**Example:**

```go
// ❌ WRONG
log.Infof("User login: email=%s, password=%s", email, password)

// ✅ CORRECT
log.Infof("user.login email=%s user_id=%s ip=%s status=success", email, userID, ipAddress)

// ❌ WRONG
log.Errorf("Token verification failed: token=%s", token)

// ✅ CORRECT
log.Errorf("user.auth_failed user_id=%s reason=invalid_token", userID)
```

---

## 9. Error Handling & Logging

### Error Codes & Responses

**Error Code Naming Convention:** `RESOURCE_ACTION_ERROR_TYPE`

```
Examples:
- USER_REGISTER_EMAIL_INVALID
- USER_LOGIN_INVALID_CREDENTIALS
- USER_PROFILE_NOT_FOUND
- AUTH_TOKEN_EXPIRED
- RATE_LIMIT_EXCEEDED
- INTERNAL_SERVER_ERROR
```

**Standard Error Response Format:**

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "USER_REGISTER_EMAIL_INVALID",
    "message": "Email format is invalid or already registered",
    "details": {
      "field": "email",
      "value": "invalid-email"
    },
    "timestamp": "2024-01-15T10:30:00Z",
    "request_id": "req_123456"  // For tracing
  }
}
```

---

### Logging Strategy: Structured Logging

**Why Structured Logs?**
- Machine-readable (JSON format)
- Searchable in log aggregation (ELK, CloudWatch, DataDog)
- Easy to filter & alert

**Log Levels:**

| Level | Use Case |
|-------|----------|
| **DEBUG** | Development only; verbose request/response details |
| **INFO** | Important milestones (user login, payment processed, deployment) |
| **WARN** | Potential issues (slow query, rate limit approaching) |
| **ERROR** | Recoverable failures (validation error, failed email send) |
| **CRITICAL** | Unrecoverable failures (database down, out of memory) |

**Implementation (Go with Zap logger):**

```go
import "go.uber.org/zap"

var logger *zap.Logger

func init() {
    var err error
    // Production: JSON format
    logger, _ = zap.NewProduction()
    // Development: Pretty print
    // logger, _ = zap.NewDevelopment()
}

// Usage
logger.Info("user login success", 
    zap.String("user_id", userID),
    zap.String("email", email),
    zap.String("ip", ipAddress),
)

logger.Error("database connection failed",
    zap.Error(err),
    zap.String("host", dbHost),
)

defer logger.Sync()  // Flush before exit
```

**Output (JSON):**

```json
{
  "level": "info",
  "ts": 1705328400.123,
  "caller": "handler/auth.go:45",
  "msg": "user login success",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "ip": "192.168.1.1"
}
```

---

### Request Tracing (Optional but recommended)

**Purpose:** Track request flow across services (important as system grows)

**Implementation with Request ID:**

```go
import "github.com/google/uuid"

func RequestIDMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        requestID := c.GetHeader("X-Request-ID")
        if requestID == "" {
            requestID = uuid.New().String()
        }
        
        c.Set("request_id", requestID)
        c.Header("X-Request-ID", requestID)
        
        logger.Info("request received",
            zap.String("request_id", requestID),
            zap.String("method", c.Request.Method),
            zap.String("path", c.Request.URL.Path),
        )
        
        c.Next()
        
        logger.Info("request completed",
            zap.String("request_id", requestID),
            zap.Int("status", c.Writer.Status()),
        )
    }
}
```

---

### Panic & Recovery

**Graceful error handling (prevent server crash):**

```go
func RecoveryMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if err := recover(); err != nil {
                logger.Error("panic recovered",
                    zap.Any("error", err),
                    zap.String("stack_trace", fmt.Sprintf("%+v", err)),
                )
                c.JSON(500, gin.H{"error": "internal server error"})
            }
        }()
        c.Next()
    }
}

// Register middleware
r.Use(RecoveryMiddleware())
```

---

## 10. Testing Strategy

### Testing Pyramid

```
           /\
          /  \ Unit Tests        (70%) - Fast, isolated, many
         /────\
        /      \  Integration    (20%) - Slower, few critical paths
       /────────\
      /          \ E2E Tests     (10%) - Slowest, user-facing happy paths
     /____________\
```

---

### Unit Tests (70% Coverage Target)

**Scope:** Test individual functions/methods in isolation

**Example: Password validation**

```go
// file: util/password_test.go
package util

import "testing"

func TestValidatePassword(t *testing.T) {
    tests := []struct {
        password string
        wantErr  bool
        errMsg   string
    }{
        {"SecurePass123!", false, ""},
        {"weak", true, "too short"},
        {"nouppercase123!", true, "missing uppercase"},
        {"NOLOWERCASE123!", true, "missing lowercase"},
        {"NoNumbers!", true, "missing number"},
        {"Nospecial123", true, "missing special character"},
    }
    
    for _, tt := range tests {
        t.Run(tt.password, func(t *testing.T) {
            err := ValidatePassword(tt.password)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidatePassword(%s) error = %v, wantErr %v", tt.password, err, tt.wantErr)
            }
        })
    }
}
```

**Run:**
```bash
go test ./util -v
```

---

### Integration Tests (20% Coverage)

**Scope:** Test business logic with real database

**Example: User registration flow**

```go
// file: tests/integration/auth_test.go
package integration

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestUserRegistration(t *testing.T) {
    // Setup: Create test DB, seed data
    db := setupTestDB(t)
    defer db.Close()
    
    router := setupRouter(db)
    
    // Test: Happy path
    payload := map[string]string{
        "email":    "newuser@example.com",
        "name":     "John Doe",
        "password": "SecurePass123!",
    }
    
    body, _ := json.Marshal(payload)
    req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    // Assertions
    if w.Code != http.StatusCreated {
        t.Errorf("expected 201, got %d", w.Code)
    }
    
    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    
    if response["success"] != true {
        t.Errorf("expected success=true")
    }
    
    // Verify DB state
    var user User
    db.QueryRow("SELECT id, email FROM users WHERE email = ?", "newuser@example.com").
        Scan(&user.ID, &user.Email)
    
    if user.Email != "newuser@example.com" {
        t.Errorf("user not inserted")
    }
}

func TestUserRegistrationDuplicateEmail(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()
    
    // Setup: Seed existing user
    db.Exec("INSERT INTO users (email, name, password_hash) VALUES (?, ?, ?)",
        "existing@example.com", "Jane", "hash123")
    
    router := setupRouter(db)
    
    // Test: Register with duplicate email
    payload := map[string]string{
        "email":    "existing@example.com",
        "name":     "John",
        "password": "SecurePass123!",
    }
    
    body, _ := json.Marshal(payload)
    req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    // Assertion
    if w.Code != http.StatusConflict {
        t.Errorf("expected 409, got %d", w.Code)
    }
}
```

**Run:**
```bash
go test ./tests/integration -v -run TestUserRegistration
```

---

### Test Data & Fixtures

**Seed test data:**

```go
// tests/fixtures/seed.sql
INSERT INTO users (id, email, name, password_hash, status, created_at)
VALUES 
    ('550e8400-e29b-41d4-a716-446655440000', 'test@example.com', 'Test User', '$2b$12$...hash...', 'active', NOW()),
    ('660e8400-e29b-41d4-a716-446655440001', 'admin@example.com', 'Admin User', '$2b$12$...hash...', 'active', NOW());

-- Load in test
func setupTestDB(t *testing.T) *sql.DB {
    db := openTestDB(t)
    
    // Run migrations
    // ...
    
    // Seed data
    fixture, _ := os.ReadFile("tests/fixtures/seed.sql")
    db.Exec(string(fixture))
    
    return db
}
```

---

### Test Coverage

```bash
# Generate coverage report
go test ./... -cover -coverprofile=coverage.out

# View HTML report
go tool cover -html=coverage.out

# Check by package
go test ./... -cover | grep coverage
```

**Target:**
- Business logic (service layer): >80%
- Overall: >70%
- Handlers: >60% (can be lower, covered by integration tests)

---

### E2E Tests (10% Coverage - Optional for MVP)

**Scope:** Full user workflows via API

**Using Postman / REST Client:**

```bash
# Save as postman_collection.json
{
  "info": { "name": "API Tests", "version": "1.0" },
  "item": [
    {
      "name": "Register User",
      "request": {
        "method": "POST",
        "url": "{{base_url}}/auth/register",
        "body": { "email": "e2e@test.com", "name": "E2E", "password": "SecurePass123!" }
      },
      "tests": [
        "pm.response.code === 201",
        "pm.response.json().data.access_token !== undefined"
      ]
    }
  ]
}
```

**Run via CLI:**
```bash
newman run postman_collection.json -e environment.json
```

---

### Test Execution

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# CI/CD (GitHub Actions)
.github/workflows/test.yml:
  - name: Run tests
    run: go test ./... -v -race -coverprofile=coverage.out
```

---

## 11. Deployment & Environment

### Environment Variables (Required)

```bash
# Database
DB_URL=postgres://user:password@host:5432/dbname
DB_MAX_CONNECTIONS=20

# Server
API_PORT=8080
API_HOST=0.0.0.0
ENV=production  # development, staging, production

# Authentication
JWT_SECRET=your_secret_key_minimum_32_characters_long
JWT_EXPIRY_HOURS=1
REFRESH_TOKEN_EXPIRY_DAYS=30

# External Services
SENDGRID_API_KEY=SG.xxxxx
AWS_S3_BUCKET=my-bucket
AWS_S3_REGION=us-east-1
AWS_ACCESS_KEY_ID=xxxx
AWS_SECRET_ACCESS_KEY=xxxx

# Logging
LOG_LEVEL=info  # debug, info, warn, error
LOG_FORMAT=json  # json, text

# Security
CORS_ORIGINS=https://example.com,https://app.example.com
RATE_LIMIT_WINDOW_MINUTES=15
RATE_LIMIT_MAX_REQUESTS=100

# (Optional) Monitoring
SENTRY_DSN=https://xxxx@sentry.io/project_id
DATADOG_API_KEY=xxxx
```

---

### Build & Run

**Local Development:**

```bash
# Install dependencies
go mod download

# Run with hot reload (using air or similar)
air

# Or manual run
go run ./cmd/api

# Verify: curl http://localhost:8080/health
```

**Docker Build (Production):**

```dockerfile
# Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app ./cmd/api

# Multi-stage: reduce final image size
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/app .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080
CMD ["./app"]
```

**Build & Run:**

```bash
# Build image
docker build -t my-app:latest .

# Run container
docker run -p 8080:8080 \
  -e DB_URL="postgres://..." \
  -e JWT_SECRET="secret" \
  my-app:latest

# Or use docker-compose for local dev
docker-compose up -d
```

**Docker Compose (Local Dev):**

```yaml
# docker-compose.yml
version: '3.8'

services:
  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      DB_URL: postgres://user:password@postgres:5432/mydb
      JWT_SECRET: dev_secret_key
      ENV: development
    depends_on:
      - postgres
    volumes:
      - .:/app  # Hot reload

  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: user
      POSTGRES_PASSWORD: password
      POSTGRES_DB: mydb
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

volumes:
  postgres_data:
```

---

### Database Migrations

**Run migrations before starting app:**

```go
// cmd/api/main.go
func main() {
    // Connect to DB
    db := connectDB()
    defer db.Close()
    
    // Run migrations
    if err := runMigrations(db); err != nil {
        log.Fatal("migration failed:", err)
    }
    
    // Start API
    router := setupRouter(db)
    router.Run(":8080")
}

func runMigrations(db *sql.DB) error {
    m := &migrate.Migrate{
        Source: "file://migrations",
        Target: "postgres://" + os.Getenv("DB_URL"),
    }
    return m.Up()
}
```

**Or via CLI (pre-deployment):**

```bash
# Render/Railway/Vercel often support this in deploy config
migrate -path ./migrations -database "$DATABASE_URL" up

# Or in shell script
./scripts/migrate.sh
```

---

### Graceful Shutdown

**Handle in-flight requests before shutting down:**

```go
func main() {
    server := &http.Server{
        Addr:    ":8080",
        Handler: router,
    }
    
    // Graceful shutdown on SIGTERM
    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
        <-sigChan
        
        logger.Info("shutdown signal received, gracefully shutting down...")
        
        // Wait up to 30 seconds for requests to complete
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        if err := server.Shutdown(ctx); err != nil {
            logger.Error("shutdown error", zap.Error(err))
        }
    }()
    
    if err := server.ListenAndServe(); err != http.ErrServerClosed {
        logger.Fatal("server error", zap.Error(err))
    }
}
```

---

### Health Check Endpoint

**Liveness probe (is service alive?):**

```go
func HealthHandler(c *gin.Context) {
    c.JSON(200, gin.H{"status": "alive"})
}

// Usage in deployment
GET /health → 200 = healthy, anything else = restart container
```

**Readiness probe (is service ready to serve requests?):**

```go
func ReadinessHandler(db *sql.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Check database connectivity
        if err := db.Ping(); err != nil {
            c.JSON(503, gin.H{"status": "not ready", "reason": "db unavailable"})
            return
        }
        
        // Check external services (optional)
        // ... check Redis, SendGrid, S3, etc
        
        c.JSON(200, gin.H{"status": "ready"})
    }
}
```

---

### Deployment Platforms

**Option 1: Render.com (Recommended for MVP)**
- ✅ Simple, auto-deploys from GitHub
- ✅ Free tier available
- ✅ Built-in database, environment variables, secrets
- ✅ Auto-scaling, SSL/TLS included

**Steps:**
1. Push to GitHub
2. Connect repo in Render dashboard
3. Set environment variables
4. Deploy (auto-redeploy on push)

**Option 2: Railway.app**
- ✅ Pay-as-you-go, very cheap for MVP
- ✅ GitHub integration
- ✅ Database included (PostgreSQL)

**Option 3: Vercel (if using Next.js)**
- ✅ Auto-deploys frontend
- ✅ Serverless functions for backend (optional)
- ✅ Free tier, generous limits

**Option 4: Traditional VPS (DigitalOcean, Linode)**
- ✅ Full control, cheaper at scale
- ❌ More manual setup (nginx, systemd, backups)
- Manual deployment script:

```bash
#!/bin/bash
# scripts/deploy.sh
set -e

echo "Deploying..."

# Pull latest code
git pull origin main

# Build
go build -o app ./cmd/api

# Migrate database
./scripts/migrate.sh

# Restart service
sudo systemctl restart myapp

echo "Deployment complete"
```

---

## 12. Observability & Monitoring

### Metrics to Collect

**Application Metrics:**
- Request count per endpoint
- Request latency (p50, p95, p99)
- Error rate (5xx, 4xx)
- Active database connections
- Cache hit rate (if using Redis)

**Business Metrics:**
- User signups per day
- Login success rate
- Email verification completion rate
- Active users

**Infrastructure Metrics:**
- CPU usage
- Memory usage
- Disk space
- Network I/O

---

### Monitoring Tools (Pick One)

**Option 1: Datadog (Enterprise, free tier limited)**
- APM, logs, metrics in one place
- Integration with most platforms
- Expensive but comprehensive

**Option 2: New Relic (Similar to Datadog)**

**Option 3: Grafana + Prometheus (Open source, self-hosted)**
```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'api'
    static_configs:
      - targets: ['localhost:8080']
```

**Option 4: Simple approach for MVP: Sentry (errors) + CloudWatch/Render logs**
```bash
# Sentry for error tracking
import "github.com/getsentry/sentry-go"

sentry.Init(sentry.ClientOptions{
    Dsn: os.Getenv("SENTRY_DSN"),
    TracesSampleRate: 0.1,
})

// Capture errors
sentry.CaptureException(err)
```

---

### Alerting Rules

**Critical (Page on-call engineer):**
- Server down (health check fails)
- Error rate >5%
- Database unavailable
- API response time p95 >1s

**Warning (Email, Slack channel):**
- Error rate >1%
- API response time p95 >500ms
- Disk space <10% remaining
- High memory usage (>80%)

---

## 13. Dependencies & Versioning

### Language & Runtime Versions

```
Go: 1.22+
Node.js: 18+ (for Next.js)
PostgreSQL: 15+
```

### Backend Dependencies (Go)

```go
// go.mod (main dependencies)
require (
    github.com/gin-gonic/gin v1.10.0              // Web framework
    github.com/lib/pq v1.10.9                     // PostgreSQL driver
    github.com/golang-jwt/jwt/v5 v5.0.0           // JWT
    golang.org/x/crypto v0.17.0                   // Bcrypt, crypto utilities
    github.com/joho/godotenv v1.5.1               // .env loading
    github.com/go-playground/validator/v10 v10.16.0  // Input validation
    go.uber.org/zap v1.27.0                       // Structured logging
    github.com/getsentry/sentry-go v0.26.0        // Error tracking
    github.com/golang-migrate/migrate/v4 v4.16.2  // Database migrations
)
```

**Rationale for each:**
- **Gin:** Minimal, fast HTTP framework (vs Echo, Fiber)
- **pq:** Pure Go PostgreSQL driver (vs GORM - simpler for MVP)
- **JWT:** Standard library for token management
- **crypto:** Password hashing (bcrypt)
- **validator:** Input validation rules
- **zap:** Structured, performant logging (vs logrus)

### Frontend Dependencies (Node.js)

```json
{
  "dependencies": {
    "next": "^14.0.0",
    "react": "^18.0.0",
    "axios": "^1.6.0"
  },
  "devDependencies": {
    "tailwindcss": "^3.4.0",
    "typescript": "^5.3.0",
    "@testing-library/react": "^14.1.0",
    "jest": "^29.7.0"
  }
}
```

---

## 14. Known Limitations & Future Work

### v1.0 Limitations (Document for transparency)

**Authentication:**
- No OAuth2 (Google, GitHub login) → defer to v1.1
- No two-factor authentication (2FA) → defer to v1.2
- Stateless JWT only (no server-side session revocation until token expires)

**Performance:**
- No caching layer (Redis) → acceptable for <1K MAU
- No CDN for frontend assets → Vercel provides basic CDN
- Single database instance (no replication)

**Features:**
- No real-time notifications → WebSocket support in v1.1
- No file uploads → defer to v1.1
- No payment processing → defer to v2.0

**Operations:**
- No automated backups (rely on DB provider)
- No multi-region deployment
- Manual scaling (horizontal scale requires redesign)

---

### v1.1 Roadmap (Examples)

**High Priority (Q1 2025):**
- [ ] OAuth2 integration (Google, GitHub)
- [ ] Email templates (HTML emails)
- [ ] User profile images (S3 upload)
- [ ] Reset password flow

**Medium Priority (Q2 2025):**
- [ ] Real-time notifications (WebSocket)
- [ ] Advanced search & filters
- [ ] Role-based admin dashboard
- [ ] API rate limiting per user tier

**Low Priority (Q3+ 2025):**
- [ ] Two-factor authentication (2FA)
- [ ] Payment integration (Stripe)
- [ ] Analytics dashboard
- [ ] Mobile app (iOS/Android)

---

## Appendix: Quick Reference

### Commands

```bash
# Development
make dev           # Run with hot reload
make test          # Run all tests
make test-coverage # Generate coverage report

# Deployment
make build         # Build Docker image
make deploy        # Deploy to Render/Railway
make migrate       # Run database migrations

# Database
make db-reset      # Drop & recreate DB (dev only)
make db-seed       # Load seed data
```

### File Checklist

```
Backend:
- ✅ cmd/api/main.go
- ✅ internal/handler/*.go
- ✅ internal/service/*.go
- ✅ internal/repository/*.go
- ✅ migrations/*.sql
- ✅ tests/integration/*.go
- ✅ docker-compose.yml
- ✅ Dockerfile
- ✅ .env.example

Frontend:
- ✅ app/page.tsx
- ✅ app/auth/login/page.tsx
- ✅ app/auth/register/page.tsx
- ✅ components/LoginForm.tsx
- ✅ lib/api.ts
- ✅ lib/auth.ts

Documentation:
- ✅ README.md (setup & usage)
- ✅ docs/API.md (endpoints)
- ✅ docs/ARCHITECTURE.md (overview)
- ✅ docs/DEPLOYMENT.md (deploy guide)
- ✅ This TDD document
```

---

**End of TDD Document**

---

## Usage Notes

1. **Customize per project:** Update technology choices, endpoints, business logic based on your actual requirements.

2. **Keep updated:** Review & update this TDD when architecture changes significantly.

3. **Reference while coding:** Use sections 5-7 (API, Logic, Security) as you implement features.

4. **Share with team:** If working with others, ensure everyone understands the design.

5. **For portfolio:** Include this TDD in your GitHub repo under `/docs/TDD.md` to impress senior engineers reviewing your code.

---

**Created:** January 2025  
**Template Version:** 2.0 (Comprehensive)
