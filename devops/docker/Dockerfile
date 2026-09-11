# Build the React application
FROM node:22-alpine AS build

WORKDIR /app

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build


# Serve the compiled application
FROM nginx:1.27-alpine

COPY devops/docker/nginx.conf /etc/nginx/conf.d/default.conf

# Vite outputs to /app/dist
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]