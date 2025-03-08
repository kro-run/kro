# Step 1: Build the Docusaurus static site
FROM node:20-alpine AS builder

# Set working directory
WORKDIR /app

# Copy only package.json and package-lock.json for caching
COPY package.json package-lock.json ./

# Install dependencies
RUN npm ci --omit=dev

# Copy the rest of the project files
COPY . .

# Build the static website
RUN npm run build

# Step 2: Use Nginx to serve the built static files
FROM nginx:alpine

# Set working directory
WORKDIR /usr/share/nginx/html

# Remove default Nginx static files
RUN rm -rf ./*

# Copy built static files from the builder stage
COPY --from=builder /app/build ./

# Expose port 80 (default for Nginx)
EXPOSE 80

# Start Nginx
CMD ["nginx", "-g", "daemon off;"]
