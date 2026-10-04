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
COPY --from=build /src/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
