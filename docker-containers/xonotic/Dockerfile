FROM debian:bullseye-slim

WORKDIR /opt

RUN apt-get update
RUN apt-get -y install python3 python3-pip

COPY Xonotic-clean .

RUN mkdir -p /opt/data && touch /opt/data/server.cfg

COPY sidecar-service/requirements.txt .
COPY sidecar-service/sidecar-service.py .
RUN pip3 install -r requirements.txt

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["python3"]
CMD ["-u", "sidecar-service.py"]
