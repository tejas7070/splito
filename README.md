<div align="center">
  <img src="frontend/src/assets/favicon.png" width="96" height="96" alt="Splito Logo" />
  <h1>Splito (Expedition Treasury Protocol)</h1>
  <p><strong>Effortless group trip expenses & zero-drama peer debt settlement powered by Go & Vue.</strong></p>

  <p>
    <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go" alt="Go Version" />
    <img src="https://img.shields.io/badge/Vue-3.4-4FC08D?style=flat&logo=vuedotjs" alt="Vue Version" />
    <img src="https://img.shields.io/badge/PrimeVue-4.0-06B6D4?style=flat" alt="PrimeVue" />
    <img src="https://img.shields.io/badge/PWA-Ready-3b82f6?style=flat" alt="PWA" />
    <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License" />
  </p>
</div>

---

## 🌟 What is Splito?

Traditional expense sharing apps treat every bill as an isolated pairwise debt ($A \text{ owes } B$), leading to tangled webs of circular payments.

**Splito** operates on an **Authoritative Central Treasury Protocol**:
1. **👑 Munimji (Treasurer)** collects advance group funds into a central pool.
2. Common expenses (tolls, fuel, hotels, meals) are logged directly from pool cash.
3. Out-of-pocket expenses paid by members are audited with real photo receipts.
4. When the journey ends, an automated **Greedy Debt Minimization Algorithm** equalizes balances, issues treasury refunds, and reduces peer transfers to at most $N - 1$.

---

## 🚀 Key Features

- **Split-Screen Trip Creation Wizard**: 3-step route planner with OpenStreetMap place suggestions, nearby locations, ordered stops, and a live driving-route map.
- **Central Treasury Management**: Real-time cash-in-hand monitoring, confirmed receipt logging, and group expense classification.
- **Peer Expense Claims & Receipt Audit**: Members submit bills with photo/PDF proofs; Munimji and Co-Reviewers review and approve in atomic transactions.
- **Fair-Share Settlement Engine (Zero-Paise-Loss)**:
  - Exact remainder allocation guaranteeing $\sum \text{shares} \equiv \text{total}$ to the exact paisa.
  - Treasurer cash treated as a balance sheet liability so $\sum \text{Net} \equiv ₹0.00$.
- **Progressive Web App (PWA) & Offline Queue**: Log transactions on highways or remote valleys without network; syncs automatically once reconnected.
- **Cryptographic PDF Statements**: Download official trip audit reports with SHA-256 verification hash stamps.
- **Enterprise-Grade Security**:
  - Hashed OTP verification (HMAC-SHA256) with brute-force lockouts.
  - Anti-CSRF protection (`X-Requested-With` enforcement and origin allowlists).
  - Role-Based Access Control (RBAC) enforced in MySQL transactions.

---

## 🛠️ Architecture & Tech Stack

```
splito/
├── backend/          # Golang (Go 1.22 + Gin Framework + MySQL)
│   ├── BLs/          # Business logic & accounting aggregations
│   ├── money/        # Deterministic split & greedy settlement algorithms
│   ├── handlers/     # REST API controllers
│   ├── middlewares/  # CSRF firewall, auth JWT verification, CORS
│   ├── services/     # Core domain workflows & email dispatches
│   └── writers/      # Prepared statements & atomic SQL transactions
├── frontend/         # Vue 3 SPA + Vite + PrimeVue 4 (Cyan Theme) + Pinia
│   ├── src/views/    # LandingView, DashboardView, LoginView
│   ├── src/stores/   # Auth, Theme, Toast, Offline Sync stores
│   └── src/assets/   # Neumorphic styling tokens & app icons
└── .github/          # Automated GitHub Actions CI/CD pipelines
```

---

## 🏁 Quick Start Guide

### Prerequisites
- [Go 1.26+](https://go.dev/dl/)
- [Node.js 20+](https://nodejs.org/)
- [MySQL 8.0+](https://dev.mysql.com/downloads/mysql/)

### 1. Clone the repository
```bash
git clone https://github.com/<your-username>/splito.git
cd splito
```

### 2. Configure Backend
```bash
cd backend
cp .env.example .env  # configure DB credentials and JWT_SECRET
go mod download
go test ./money/... ./middlewares/... ./services/... -v
go run main.go
```

### 3. Configure Frontend
```bash
cd ../frontend
npm install
npm run dev
```

Visit `http://localhost:5173` (or `http://localhost:8080` if served through the Go server).

Location suggestions use OpenStreetMap data via Photon and Overpass; route previews use OSRM and OpenStreetMap tiles. These public services require an internet connection and may be rate-limited. Map data attribution is shown in the UI.

---

## 🧪 Testing

Run backend unit and property-based test suites:
```bash
cd backend
go test ./money/... ./middlewares/... ./services/... -v
```

---

## 📄 License
This project is licensed under the [MIT License](LICENSE).
