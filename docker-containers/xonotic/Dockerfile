FROM debian:bookworm-slim

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

WORKDIR /opt

RUN apt-get update

COPY Xonotic-clean .

RUN mkdir -p /opt/data && touch /opt/data/server.cfg

COPY server.cfg /opt/default-server.cfg
COPY sidecar-service/pyproject.toml .
COPY sidecar-service/sidecar-service.py .
RUN uv sync --no-install-project --python 3.12.3

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/opt/.venv/bin/python"]
CMD ["-u", "sidecar-service.py"]
