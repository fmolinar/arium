# lets use a small alpine image for our node app
FROM node:22-alpine 

# Set the working directory to /app
WORKDIR /app

# Copy package.json and package-lock.json to the working directory
COPY arium/package.json arium/package-lock.json ./
# Install dependencies
RUN npm ci
# Copy the rest of the application code to the working directory
COPY arium/ ./
# Build the application
RUN npm run build

# Expose the port that the application will run on
EXPOSE 4173

# Start the application
CMD ["npm", "run", "preview", "--", "--host", "0.0.0.0"]