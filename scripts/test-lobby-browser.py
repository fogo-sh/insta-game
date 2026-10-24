"""Check launch-page retries in Chromium without AWS credentials."""

import argparse
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread

from playwright.sync_api import expect, sync_playwright

parser = argparse.ArgumentParser()
parser.add_argument("--url", help="Use deployed assets instead of the local web directory; API calls stay mocked")
args = parser.parse_args()


class QuietHandler(SimpleHTTPRequestHandler):
    def log_message(self, *_):
        pass


server = None
if not args.url:
    directory = Path(__file__).resolve().parents[1] / "internal/lobby/web"
    server = ThreadingHTTPServer(("127.0.0.1", 0), partial(QuietHandler, directory=str(directory)))
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
                game["pending"] = True
                route.fulfill(status=202, json={"message": "Start requested"})

        page.route("**/api/**", api)
        page.goto(args.url)
        page.get_by_label("Invitation key").fill("browser-test-key")
        page.get_by_role("button", name="Continue", exact=True).click()
        start = page.get_by_role("button", name="Start", exact=True)
        expect(start).to_be_enabled()
        start.click()
        expect(page.locator("#message")).to_have_text("Try again", timeout=10000)
        expect(start).to_be_enabled()
        assert attempts == 3, f"unbounded retries: {attempts}"

        mode, attempts = "retry", 0
        start.click()
        expect(page.locator("#message")).to_have_text("Start requested", timeout=10000)
        expect(page.get_by_role("button", name="Starting…", exact=True)).to_be_disabled()
        assert attempts == 2, f"did not recover from one throttle: {attempts}"
        page.get_by_role("button", name="Sign out").click()
        expect(page.get_by_label("Invitation key")).to_be_visible()
        expect(page.locator("article")).to_have_count(0)
        servers = []
        create_ids = []

        def manage(route):
            request = route.request
            if request.method == "GET":
                route.fulfill(json={"host": "running", "games": servers, "presets": [{"id": "quake", "name": "Quake"}]})
            elif request.url.endswith("/api/games"):
                body = request.post_data_json
                create_ids.append(body["id"])
                if len(create_ids) == 1:
                    route.fulfill(status=503, json={"error": "Busy"})
                    return
                if not any(server["id"] == body["id"] for server in servers):
                    servers.append({"id": body["id"], "name": body["name"], "game": "Quake", "phase": "running", "desired": "running", "players": 0, "address": f"192.0.2.1:{26000 + len(servers)}"})
                route.fulfill(status=202, json={"message": "Server requested"})
            else:
                server_id, action = request.url.split("/")[-2:]
                server = next(server for server in servers if server["id"] == server_id)
                server.update(desired={"stop": "stopped", "start": "running", "remove": "deleted"}[action], phase={"stop": "stopped", "start": "running", "remove": "deleted"}[action])
                route.fulfill(status=202, json={"message": "Request saved"})

        page.unroute("**/api/**", api)
        page.route("**/api/**", manage)
        page.get_by_label("Invitation key").fill("browser-test-key")
        page.get_by_role("button", name="Continue", exact=True).click()
        expect(page.get_by_label("Game", exact=True)).to_be_visible()
        for name in ["Friday", "Saturday"]:
            page.get_by_label("Server name", exact=True).fill(name)
            page.get_by_role("button", name="Start server", exact=True).click()
            expect(page.get_by_role("heading", name=name, exact=True)).to_be_visible(timeout=10000)
        assert create_ids[0] == create_ids[1] and create_ids[1] != create_ids[2], create_ids
        expect(page.locator("article")).to_have_count(2)
        friday = page.locator("article").filter(has=page.get_by_role("heading", name="Friday", exact=True))
        saturday = page.locator("article").filter(has=page.get_by_role("heading", name="Saturday", exact=True))
        friday.get_by_role("button", name="Stop", exact=True).click()
        expect(friday.get_by_role("button", name="Start", exact=True)).to_be_enabled()
        expect(saturday.get_by_role("button", name="Stop", exact=True)).to_be_enabled()
        page.on("dialog", lambda dialog: dialog.accept())
        friday.get_by_role("button", name="Remove", exact=True).click()
        expect(page.locator("article")).to_have_count(1)
        expect(saturday).to_be_visible()
        page.set_viewport_size({"width": 390, "height": 844})
        assert page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "mobile page overflows"
        assert not errors, errors
        browser.close()
        print("Launch-page retries, independent servers, stop/remove, mobile layout, and sign-out passed")
finally:
    if server:
        server.shutdown()
        server.server_close()
        thread.join()
