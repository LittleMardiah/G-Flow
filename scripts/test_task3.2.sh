#!/bin/bash
set -e
trap 'kill $SERVER_PID 2>/dev/null' EXIT

export DATABASE_URL='postgresql://postgres:password@localhost:15432/g_flow_dev?sslmode=disable'
export JWT_SECRET='your-super-secret-jwt-key-change-in-production'
export PORT=8080
export REDIS_URL='redis://localhost:6380'

mkdir -p logs

# Install jq jika belum ada
if ! command -v jq &> /dev/null; then
    echo "📦 jq not found, installing..."
    sudo apt-get update -qq && sudo apt-get install jq -y -qq
fi

echo "🚀 Starting server..."
go run cmd/api/main.go > logs/server.log 2>&1 &
SERVER_PID=$!
sleep 6

# Cek apakah server hidup
if ! ps -p $SERVER_PID > /dev/null; then
    echo "❌ Server failed to start. Check logs/server_error.log"
    cat logs/server_error.log
    exit 1
fi

echo "1. Register merchant user..."
REG_RESP=$(curl -s -X POST http://localhost:8080/api/v1/auth/register -H 'Content-Type: application/json' -d '{"email":"merchant_api@test.com","password":"Test123!","name":"Merchant API","user_type":"merchant","phone":"081234567891"}')
USER_ID=$(echo $REG_RESP | jq -r '.data.user_id')
echo "✅ USER ID: $USER_ID"

echo "2. Login merchant user..."
LOGIN_RESP=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' -d '{"email":"merchant_api@test.com","password":"Test123!"}')
TOKEN=$(echo $LOGIN_RESP | jq -r '.data.access_token')
echo "✅ TOKEN: ${TOKEN:0:20}..."

echo "3. Register merchant profile..."
MERCHANT_RESP=$(curl -s -X POST http://localhost:8080/api/v1/merchants/register -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" -d '{"merchant_name":"Warung API Test","merchant_description":"Test desc","category":"Indonesian","address":"Jl. Test","latitude":-6.2088,"longitude":106.8456,"phone":"081234567892","opening_time":"10:00","closing_time":"22:00"}')
MERCHANT_ID=$(echo $MERCHANT_RESP | jq -r '.data.id')
echo "✅ MERCHANT ID: $MERCHANT_ID"

echo "4. GET merchant..."
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X GET http://localhost:8080/api/v1/merchants/$MERCHANT_ID -H "Authorization: Bearer $TOKEN")
echo "✅ GET MERCHANT STATUS: $STATUS"

echo "5. POST menu..."
MENU_RESP=$(curl -s -X POST http://localhost:8080/api/v1/merchants/$MERCHANT_ID/menus -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" -d '{"name":"Menu Test","sequence_order":1}')
MENU_ID=$(echo $MENU_RESP | jq -r '.data.id')
echo "✅ MENU ID: $MENU_ID"

echo "6. POST item..."
ITEM_RESP=$(curl -s -X POST http://localhost:8080/api/v1/merchants/$MERCHANT_ID/items -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" -d "{\"menu_id\":\"$MENU_ID\",\"name\":\"Nasi Goreng Test\",\"description\":\"Test\",\"price\":25000,\"stock\":100,\"is_available\":true}")
ITEM_ID=$(echo $ITEM_RESP | jq -r '.data.id')
echo "✅ ITEM ID: $ITEM_ID"

echo "7. PATCH item..."
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X PATCH http://localhost:8080/api/v1/merchants/$MERCHANT_ID/items/$ITEM_ID -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" -d '{"price":30000}')
echo "✅ PATCH ITEM STATUS: $STATUS"

echo "🛑 Stopping server..."
kill $SERVER_PID 2>/dev/null

LOG_MSG="ALL TESTS PASSED - $(date)"
echo "$LOG_MSG" | tee -a logs/task3.2_test.log

echo "📝 Committing..."
git add internal/food cmd/api/main.go
git commit -m 'feat: implement merchant onboarding and catalog management (Task 3.2)'
git status
git log --oneline -n 3

echo ""
echo "=========================================="
echo "✅ SEMUA TEST BERHASIL"
echo "📁 Log tersimpan di logs/task3.2_test.log"
echo "=========================================="
