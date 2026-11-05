FROM node:24-alpine AS build
WORKDIR /src
COPY package.json pnpm-lock.yaml .npmrc ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY . ./
ARG VITE_PUBLIC_URL=
ARG VITE_CORRECTION_CONTACT=info@subcult.tv
ENV VITE_PUBLIC_URL=$VITE_PUBLIC_URL VITE_CORRECTION_CONTACT=$VITE_CORRECTION_CONTACT
RUN pnpm run build

FROM nginx:1.29-alpine
ARG CORE_UPSTREAM=api:8080
COPY --from=build /src/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
RUN case "$CORE_UPSTREAM" in ''|*[!a-zA-Z0-9._:-]*) exit 1;; esac \
    && sed -i "s#api:8080#${CORE_UPSTREAM}#g" /etc/nginx/conf.d/default.conf
EXPOSE 80
