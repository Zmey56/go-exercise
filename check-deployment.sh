#!/bin/bash

echo "🔍 Checking Docker Compose Deployment..."
echo "========================================"

# Check if docker-compose is running
echo "1. Checking container status..."
docker-compose ps

echo -e "\n2. Checking health endpoints..."
echo "Health check:"
curl -s http://localhost:8080/health | jq '.' || echo "❌ Health check failed"

echo -e "\nReadiness check:"
curl -s http://localhost:8080/ready | jq '.' || echo "❌ Readiness check failed"

echo -e "\n3. Testing API endpoints..."
echo "All pairs:"
curl -s http://localhost:8080/api/v1/ltp | jq '.' || echo "❌ API test failed"

echo -e "\nSingle pair (BTC/USD):"
curl -s "http://localhost:8080/api/v1/ltp?pair=BTC/USD" | jq '.' || echo "❌ Single pair test failed"

echo -e "\n4. Checking Redis..."
echo "Redis ping:"
docker-compose exec -T redis redis-cli ping || echo "❌ Redis ping failed"

echo -e "\nRedis keys:"
docker-compose exec -T redis redis-cli keys "*" || echo "❌ Redis keys check failed"

echo -e "\n5. Checking cache metrics..."
echo "Cache metrics:"
curl -s http://localhost:8080/metrics | grep -E "(cache_|redis_)" || echo "❌ Cache metrics not found"

echo -e "\n6. Checking logs for errors..."
echo "Recent errors in ltp-api:"
docker-compose logs --tail=10 ltp-api | grep -i error || echo "✅ No recent errors"

echo -e "\nRecent errors in redis:"
docker-compose logs --tail=10 redis | grep -i error || echo "✅ No recent errors"

echo -e "\n✅ Deployment check completed!"

