FROM node:22-alpine

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install --production=false

COPY . .
RUN npm run build

EXPOSE 8003

CMD ["node", "dist/app.js"]
