FROM debian:latest
RUN apt-get update
RUN apt-get -y install aria2 unzip python3 python3-pip
RUN aria2c --seed-time=0 https://dl.xonotic.org/xonotic-0.8.2.zip.torrent
RUN unzip xonotic-0.8.2.zip
RUN rm xonotic-0.8.2.zip
RUN mkdir /app
RUN mv Xonotic/* /app
COPY ./requirements.txt /app/requirements.txt
WORKDIR /app
RUN pip install -r requirements.txt
COPY . /app
ENTRYPOINT ["python"]
CMD ["service.py"]
