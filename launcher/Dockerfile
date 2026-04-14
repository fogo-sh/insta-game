# Stage 1: build
FROM node:22-alpine AS build
WORKDIR /build
COPY package.json package-lock.json ./
RUN npm ci
COPY src/ ./src/
COPY tsconfig.json ./
RUN npm run build:docker

# Stage 2: runtime
FROM node:22-alpine
WORKDIR /app
COPY --from=build /build/dist/ ./dist/
ENV NODE_ENV=production
EXPOSE 3000
CMD ["node", "dist/server.js"]
