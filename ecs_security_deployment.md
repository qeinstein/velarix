# Amazon ECS: Security & Deployment Strategy

For your deployment on **Amazon ECS**, follow these best practices to ensure data integrity and prevent exploitation.

## 🛡️ Preventing Exploitation (Hardening)

### 1. External Security (AWS Layer)
- **Application Load Balancer (ALB)**: Never expose the Go server directly to the internet. Use an ALB to handle **TLS/SSL Termination**. This protects your API from man-in-the-middle attacks.
- **Security Groups**: Configure your ECS task security group to **only** allow traffic on port 80/8080 from your ALB's security group. Drain all other traffic.
- **AWS WAF**: Attach a Web Application Firewall (WAF) to your ALB to prevent common attacks like SQLi (not a risk here, but good practice), XSS, and bot-driven brute force.

### 2. Internal Security (Velarix Layer)
- **Tenant Isolation**: Velarix already enforces `OrgID` on every request. This ensures that even if an attacker gains an API key for Tenant A, they can **never** see data from Tenant B.
- **Rate Limiting**: The built-in 60 RPM limit (persisted in BadgerDB) prevents brute-force attempts.
- **Encryption-at-Rest**: Set `VELARIX_ENV=prod` to ensure the server refuses to boot without your 32-byte AES-256 key.

### 3. Secrets Management
**Never** hardcode your keys in the ECS Task Definition.
- **Method**: Store `VELARIX_ENCRYPTION_KEY` and `VELARIX_API_KEY` in **AWS Secrets Manager**.
- **Integration**: Map these secrets to environment variables in your ECS Task Definition. This ensures they are injected into the container at runtime and never logged.

## 🚀 ECS Architecture Checklist

### 💾 Persistent Storage (Amazon EFS)
For ECS Fargate (the most common ECS mode), you should use **Amazon EFS** for your `velarix.data` directory. 
- **Setup**: Create an EFS file system and an Access Point.
- **Mount**: In your ECS Task Definition, create a volume pointing to the EFS Access Point and mount it to `/root/velarix.data` in the container.
- **Why?**: This ensures your database survives container restarts or task migrations across different availability zones.

### 📜 Logging & Monitoring
- **CloudWatch Logs**: The `slog` output is already JSON-formatted. Ensure your ECS task sends these to CloudWatch to enable searching and alerting for security errors.
- **IAM Roles**: Use an **ECS Task Role** to give your application the minimum permissions it needs (e.g., access to EFS and Secrets Manager).

### 🛠️ ECR Repository
1.  **Build**: `docker build -t velarix-server .`
2.  **Push**: Push this image to **Amazon ECR**.
3.  **Deploy**: Update your ECS Service to pull from ECR.
