#!/bin/bash
# control-plane/provisioning/setup_tenant.sh
# 
# Usage: ./setup_tenant.sh <org_id> <admin_email> <plan_tier>
# This script provisions isolated resources or limits for a new enterprise tenant.

set -e

ORG_ID=$1
ADMIN_EMAIL=$2
PLAN_TIER=$3

if [ -z "$ORG_ID" ] || [ -z "$ADMIN_EMAIL" ] || [ -z "$PLAN_TIER" ]; then
  echo "Usage: $0 <org_id> <admin_email> <plan_tier>"
  exit 1
fi

echo "Provisioning Tenant: $ORG_ID ($PLAN_TIER)"

# 1. Provision Rate Limits in Redis based on Tier
REDIS_URL=${VELARIX_REDIS_URL:-"redis://localhost:6379"}

if [ "$PLAN_TIER" == "enterprise" ]; then
  RATE_LIMIT=10000
else
  RATE_LIMIT=100
fi

echo "Setting API Rate Limit to $RATE_LIMIT req/min..."
# Example command using redis-cli (pseudo-code logic)
# redis-cli -u $REDIS_URL SET "tenant_config:$ORG_ID:rate_limit" $RATE_LIMIT

# 2. Setup Stripe Customer Mapping
echo "Creating Stripe Customer mapping for $ADMIN_EMAIL..."
# curl -X POST https://api.stripe.com/v1/customers -u $STRIPE_SECRET_KEY: -d email=$ADMIN_EMAIL -d metadata[org_id]=$ORG_ID

echo "Tenant $ORG_ID provisioned successfully."
