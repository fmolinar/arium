FROM node:22-alpine

WORKDIR /app

COPY arium/package.json arium/package-lock.json ./
RUN npm ci

COPY arium/ ./
RUN npm run build

EXPOSE 4173

CMD ["npm", "run", "preview", "--", "--host", "0.0.0.0"]