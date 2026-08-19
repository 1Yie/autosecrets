FROM python:3.13-slim
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 \
	PYTHONUNBUFFERED=1 \
	TZ=Asia/Shanghai
COPY pyproject.toml ./
COPY src ./src
COPY tests/devproxy.py /opt/devproxy.py
COPY docker-entrypoint.sh /entrypoint.sh
RUN pip install --no-cache-dir . \
	&& chmod 0755 /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
