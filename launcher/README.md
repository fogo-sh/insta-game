# Launcher Service

## Routes
* GET `/`
  * Shows a HTML page with server info
    * 
-----
* GET `/game`
  * Returns game state in JSON
* POST `/game`
  * Creates a new game and returns game state in JSON
* DELETE `/game`
  * Kills an existing game instance
```
$ curl -X GET http://127.0.0.1:5000/game
{
  "public_ip": "35.182.245.71", 
  "status": "online"
}


$ curl -X POST http://127.0.0.1:5000/game
{
  "public_ip": "35.183.197.249", 
  "status": "online"
}


$ curl -X DELETE http://127.0.0.1:5000/game
{
  "status": "offline"
}
```
----
* GET  `/config`
  * Returns a predefined configuration file. Defaults to `xonotic_config_file`
* POST `/config` with parameter `config`
  * Downloads file specified by `config`, and replaces default file returned by GET /config
```
$ curl -X GET http://127.0.0.1:5000/config  > out.txt; head -n5 out.txt
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100 15161  100 15161    0     0  12.4M      0 --:--:-- --:--:-- --:--:-- 14.4M
// This is an example config, to actually use it, copy it to ~/.xonotic/data on linux
// or the equivalent directory on your OS (https://xonotic.org/faq/#config).

// Two slashes start a comment until the end of the line.
// Surround settings with double quotes (e.g. cvar_name "some value") unless they're a single number or word.


$ curl -X POST -F config=http://www.google.ca http://127.0.0.1:5000/config
{
  "status": "success"
}

```

