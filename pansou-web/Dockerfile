# ---- 构建阶段 ----
FROM node:20-alpine AS build
WORKDIR /app

# 先装依赖（利用 Docker 层缓存）
COPY package*.json ./
RUN npm install --registry https://registry.npmmirror.com

# 再拷源码并构建
COPY . .
RUN npm run build

# ---- 运行阶段 ----
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
