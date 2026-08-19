FROM python:3.13-slim
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 \
	PYTHONUNBUFFERED=1 \
	TZ=Asia/Shanghai \
	AGENT_SRC=/app
COPY pyproject.toml ./
COPY src ./src
COPY tests/devproxy.py /opt/devproxy.py
COPY docker-entrypoint.sh /entrypoint.sh
COPY docker/build-artifacts.sh /usr/local/bin/build-artifacts
RUN apt-get update \
	&& apt-get install -y --no-install-recommends openssl \
	&& rm -rf /var/lib/apt/lists/* \
	&& pip install --no-cache-dir . \
	&& chmod 0755 /entrypoint.sh /usr/local/bin/build-artifacts
ENTRYPOINT ["/entrypoint.sh"]
