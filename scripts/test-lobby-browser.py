"""Check launch-page retries in Chromium without AWS credentials."""

import argparse
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread
from urllib.parse import urlparse

from playwright.sync_api import expect, sync_playwright

parser = argparse.ArgumentParser()
parser.add_argument(
    "--url",
    help="Use deployed assets instead of the local web directory; API calls stay mocked",
)
args = parser.parse_args()


class QuietHandler(SimpleHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/status.js":
            data = (Path(__file__).resolve().parents[1] / "internal/serverui/status.js").read_bytes()
            self.send_response(200)
            self.send_header("Content-Type", "text/javascript")
            self.end_headers()
            self.wfile.write(data)
            return
        super().do_GET()

    def log_message(self, *_):
        pass


server = None
if not args.url:
    directory = Path(__file__).resolve().parents[1] / "internal/launchui/web"
    server = ThreadingHTTPServer(
        ("127.0.0.1", 0), partial(QuietHandler, directory=str(directory))
    )
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    args.url = f"http://127.0.0.1:{server.server_port}"

try:
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch()
        page = browser.new_page()
        errors = []
        page.on("pageerror", lambda error: errors.append(str(error)))
        game = {
            "id": "friday",
            "name": "Friday games",
            "game": "BZFlag",
            "phase": "sleeping",
            "desired": "stopped",
            "pending": False,
        }
        attempts = 0
        mode = "busy"

        def api(route):
            global attempts
            if route.request.method == "GET":
                route.fulfill(json={"host": "stopped", "games": [game]})
                return
            attempts += 1
            if mode == "busy" or attempts == 1:
                route.fulfill(status=429, json={"error": "Try again"})
            else:
                game.update(
                    pending=True, desired="running", phase="preparing", action="start"
                )
                route.fulfill(status=202, json={"message": "Start requested"})

        page.route("**/api/**", api)
        page.goto(args.url)
        page.get_by_label("Invitation key").fill("browser-test-key")
        page.get_by_role("button", name="Open", exact=True).click()
        start = page.get_by_role("button", name="Start", exact=True)
        expect(page.locator("#host")).to_have_text("1 server")
        expect(start).to_be_enabled()
        start.click()
        expect(page.locator("#message")).to_have_text("Try again", timeout=10000)
        expect(start).to_be_enabled()
        assert attempts == 3, f"unbounded retries: {attempts}"

        mode, attempts = "retry", 0
        start.click()
        expect(page.locator("#message")).to_have_text("Start requested", timeout=10000)
        expect(page.get_by_role("button", name="Stop", exact=True)).to_be_enabled()
        assert attempts == 2, f"did not recover from one throttle: {attempts}"
        page.get_by_role("button", name="Sign out").click()
        expect(page.get_by_label("Invitation key")).to_be_visible()
        expect(page.locator("article")).to_have_count(0)
        expect(page.locator("#host")).to_be_hidden()
        servers = []
        create_ids = []

        def manage(route):
            request = route.request
            if request.method == "GET":
                route.fulfill(
                    json={
                        "hosts": [
                            {"id": "host-a", "name": "Host 1", "state": "running"}
                        ],
                        "games": servers,
                        "choices": [
                            {"id": "fteqw", "name": "Quake"},
                            {"id": "bzflag", "name": "BZFlag"},
                        ],
                    }
                )
            elif request.url.endswith("/api/games"):
                body = request.post_data_json
                assert (
                    body["game"] in ["fteqw", "bzflag"]
                    and "template" not in body
                    and "host" not in body
                )
                create_ids.append(body["id"])
                if len(create_ids) == 1:
                    route.fulfill(status=503, json={"error": "Busy"})
                    return
                if not any(server["id"] == body["id"] for server in servers):
                    servers.append(
                        {
                            "id": body["id"],
                            "name": body["name"],
                            "hostId": "host-a",
                            "game": "Quake",
                            "phase": "running",
                            "desired": "running",
                            "players": 0,
                            "address": f"192.0.2.1:{26000 + len(servers)}",
                        }
                    )
                route.fulfill(status=202, json={"message": "Server requested"})
            else:
                server_id, action = request.url.split("/")[-2:]
                server = next(server for server in servers if server["id"] == server_id)
                server.update(
                    desired={
                        "stop": "stopped",
                        "start": "running",
                        "remove": "deleted",
                    }[action],
                    phase={"stop": "stopped", "start": "running", "remove": "deleted"}[
                        action
                    ],
                )
                route.fulfill(status=202, json={"message": "Request saved"})

        page.unroute("**/api/**", api)
        page.route("**/api/**", manage)
        page.get_by_label("Invitation key").fill("browser-test-key")
        page.get_by_role("button", name="Open", exact=True).click()
        expect(page.get_by_label("Game", exact=True)).to_be_visible()
        expect(page.locator("#host")).to_have_text("0 servers")
        expect(page.locator("#preset option")).to_have_count(2)
        for name in ["Friday", "Saturday"]:
            page.locator("#name").fill(name)
            page.get_by_role("button", name="Start server", exact=True).click()
            expect(page.get_by_role("heading", name=name, exact=True)).to_be_visible(
                timeout=10000
            )
        assert create_ids[0] == create_ids[1] and create_ids[1] != create_ids[2], (
            create_ids
        )
        expect(page.locator("article")).to_have_count(2)
        friday = page.locator("article").filter(
            has=page.get_by_role("heading", name="Friday", exact=True)
        )
        saturday = page.locator("article").filter(
            has=page.get_by_role("heading", name="Saturday", exact=True)
        )
        friday.get_by_role("button", name="Stop", exact=True).click()
        expect(friday.get_by_role("button", name="Start", exact=True)).to_be_enabled()
        expect(saturday.get_by_role("button", name="Stop", exact=True)).to_be_enabled()
        page.on("dialog", lambda dialog: dialog.accept())
        friday.get_by_role("button", name="Remove", exact=True).click()
        expect(page.locator("article")).to_have_count(1)
        expect(saturday).to_be_visible()
        page.set_viewport_size({"width": 390, "height": 844})
        assert page.evaluate("document.documentElement.scrollWidth <= innerWidth"), (
            "mobile page overflows"
        )
        # Exercise the controller detail with the same embedded status module.
        controller = browser.new_page()
        controller.on("pageerror", lambda error: errors.append(str(error)))
        root = Path(__file__).resolve().parents[1]
        controller_dir = root / "internal/control/web"
        assets = {"/": "index.html", "/app.js": "app.js", "/settings.js": "settings.js", "/style.css": "style.css"}
        current = {
            "id": "test", "name": "Friday", "game": {"id": "fteqw", "displayName": "Quake", "gamePort": 26000},
            "provider": "docker", "desired": "running", "phase": "running", "players": 0,
            "endpoints": [{"port": 26000, "address": "192.0.2.1:26000"}], "createdAt": "2026-11-11T00:00:00Z",
            "idleMinutes": 0, "maxMinutes": 0,
        }
        operations = []

        def controller_route(route):
            path = urlparse(route.request.url).path
            if path in assets:
                content_type = "text/javascript" if path.endswith(".js") else "text/css" if path.endswith(".css") else "text/html"
                route.fulfill(body=(controller_dir / assets[path]).read_bytes(), content_type=content_type)
            elif path == "/status.js":
                route.fulfill(body=(root / "internal/serverui/status.js").read_bytes(), content_type="text/javascript")
            elif path == "/api/catalog":
                route.fulfill(json=[])
            elif path == "/api/providers":
                route.fulfill(json={"providers": ["docker"], "default": "docker"})
            elif route.request.method == "GET":
                route.fulfill(json=[current])
            else:
                operations.append((route.request.method, path))
                route.fulfill(json=current)

        controller.route("**/*", controller_route)
        controller.add_init_script("sessionStorage.setItem('insta-game-token', 'test-token')")
        controller.goto(args.url + "/#server/test")
        expect(controller.locator("#fact-status")).to_have_text("Running")
        expect(controller.locator("#copy-address")).to_be_enabled()
        console_input = controller.get_by_role("textbox", name="command", exact=True)
        console_input.fill("status")
        current.update(desired="stopped", error="Could not stop", stopReason="Maximum runtime reached")
        expect(controller.locator("#fact-status")).to_have_text("Failed", timeout=10000)
        expect(controller.get_by_role("button", name="Retry stop", exact=True)).to_be_visible()
        expect(controller.locator("#copy-address")).to_be_disabled()
        expect(console_input).to_have_value("status")
        expect(controller.locator("#stop-reason")).to_have_text("Maximum runtime reached")
        current.update(desired="running", phase="failed", error="Game exited", stopReason="")
        expect(controller.get_by_role("button", name="Retry", exact=True)).to_be_visible(timeout=10000)
        expect(controller.locator("#stop-reason")).to_have_text("idle timeout: off; runtime limit: off")
        controller.get_by_role("button", name="Retry", exact=True).click()
        assert operations[-1] == ("POST", "/api/instances/test/start"), operations
        current.update(phase="starting", error="")
        expect(controller.locator("#fact-status")).to_have_text("Starting...", timeout=10000)
        current.update(desired="deleted", error="Could not remove")
        expect(controller.get_by_role("button", name="Retry remove", exact=True)).to_be_visible(timeout=10000)
        expect(controller.locator("#fact-status")).to_have_text("Failed")
        for width in [390, 768, 1440]:
            controller.set_viewport_size({"width": width, "height": 900})
            assert controller.evaluate("document.documentElement.scrollWidth <= innerWidth"), width
        controller.close()
        assert not errors, errors
        browser.close()
        print(
            "Launch and controller lifecycle controls, retries, console retention, responsive layout, and sign-out passed"
        )
finally:
    if server:
        server.shutdown()
        server.server_close()
        thread.join()
