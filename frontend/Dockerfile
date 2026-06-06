# Multi-stage Dockerfile for the SocialSentry frontend

# === Development stage (used by docker-compose for local dev) ===
FROM node:20-alpine AS development
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 3000
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]

# === Builder stage ===
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# === Production stage ===
FROM nginx:alpine AS production
# API_UPSTREAM is substituted into the nginx template at container start.
# Override it (e.g. to a project-prefixed hostname) via the environment if needed.
ENV API_UPSTREAM=api:8080
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
