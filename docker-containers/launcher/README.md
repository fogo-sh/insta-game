# Launcher Service

## Routes
* GET `/`
  * Shows a HTML page with server info
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
