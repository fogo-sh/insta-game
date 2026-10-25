(() => {
  const data = JSON.parse(document.getElementById("history-data").textContent);
  if (!data.runs.length) return;
  const el = (id) => document.getElementById("history-" + id);
  const colors = {
    pass: "#9dcc8a",
    pressure: "#e3b45d",
    fail: "#ec897d",
    check: "#8896af",
  };
  const outcomes = {
    pass: "Passed",
    pressure: "Pressure / screening threshold",
    fail: "Failed test attempt",
    check: "Validation / observer check",
  };
  const names = {
    fteqw: "Quake I",
    q2repro: "Quake II",
    ioquake3: "Quake III",
    openarena: "OpenArena",
    ut99: "UT99",
    xonotic: "Xonotic",
    bzflag: "BZFlag",
  };
  const hostName = (h) =>
    ({
      "t4g.micro": "Micro · 1 GiB",
      "t4g.small": "Small · 2 GiB",
      "t4g.medium": "Medium · 4 GiB",
      "t4g.small.q2": "Small · Q2 repeat",
      "t4g.small.mix": "Small · mixed games",
    })[h] || h;
  const time = d3.utcFormat("%H:%M");
  const stamp = (seconds) =>
    d3.utcFormat("%H:%M:%S UTC")(new Date(seconds * 1000));
  const format = (value, digits = 2) =>
    value == null ? "not captured" : Number(value).toFixed(digits);
  const sessions = [];
  for (const run of data.runs) {
    let s = sessions.at(-1);
    if (!s || run.start - s[1] > 1800)
      sessions.push((s = [run.start, run.end]));
    s[1] = Math.max(s[1], run.end, run.start + 1);
  }
  sessions.forEach((s) => {
    s[0] -= 30;
    s[1] += 60;
  });
  const all = [d3.min(sessions, (d) => d[0]), d3.max(sessions, (d) => d[1])];
  sessions.forEach((s, i) =>
    el("session").add(
      new Option(
        "Session " +
          (i + 1) +
          " · " +
          time(new Date(s[0] * 1000)) +
          "–" +
          time(new Date(s[1] * 1000)),
        String(i),
      ),
    ),
  );
  if (sessions.length > 1) el("session").add(new Option("All sessions", "all"));
  [...new Set(data.runs.flatMap((r) => r.games))]
    .sort()
    .forEach((g) => el("game").add(new Option(names[g] || g, g)));
  const samples = d3.group(data.samples, (d) => d[0]);
  const state = {
    session: sessions.length - 1,
    host: data.hosts[0].tier,
    game: "",
    window: null,
    selected: null,
  };
  el("session").value = String(state.session);
  const scope = () => (state.session === "all" ? all : sessions[state.session]);
  const domain = () => state.window || scope();
  const overlaps = (r, d) => r.end >= d[0] && r.start <= d[1];
  const tip = el("tip");
  function tooltip(event, text) {
    tip.hidden = false;
    tip.textContent = text;
    tip.style.left =
      Math.max(8, Math.min(event.clientX + 14, window.innerWidth - 350)) + "px";
    tip.style.top =
      Math.max(
        8,
        Math.min(
          event.clientY + 14,
          window.innerHeight - tip.offsetHeight - 12,
        ),
      ) + "px";
  }
  function describe(r) {
    return (
      r.games.map((g) => names[g] || g).join(" + ") +
      " · " +
      r.servers +
      " server" +
      (r.servers === 1 ? "" : "s") +
      " · " +
      r.bots +
      " bots/server · " +
      r.clients +
      " network players total"
    );
  }
  const emptyInspector = el("inspector").cloneNode(true);
  function inspect(r) {
    const box = el("inspector");
    box.replaceChildren();
    box.style.borderColor = colors[r.outcome];
    const title = document.createElement("strong");
    title.textContent = outcomes[r.outcome] + " — " + describe(r);
    box.append(title);
    const timing = document.createElement("p");
    timing.textContent =
      r.label +
      " · " +
      hostName(r.host) +
      " · " +
      stamp(r.start) +
      " → " +
      (r.end_known
        ? stamp(r.end)
        : "last observed " + stamp(r.end) + "; finish not captured");
    box.append(timing);
    const metrics = document.createElement("p");
    metrics.textContent =
      "p95 CPU " +
      format(r.cpu) +
      " cores · free RAM p05 " +
      (r.free == null ? "not captured" : format(r.free * 100, 1) + "%") +
      " · query p95 " +
      format(r.latency, 1) +
      " ms · query loss " +
      (r.loss == null ? "not captured" : format(r.loss * 100, 1) + "%");
    box.append(metrics);
    if (r.reason) {
      const reason = document.createElement("p");
      reason.textContent = r.reason;
      box.append(reason);
    }
  }
  function choose(r) {
    state.selected = r;
    state.host = r.host;
    el("host").value = r.host;
    const s = scope();
    state.window = [
      Math.max(s[0], r.start - 30),
      Math.min(s[1], Math.max(r.end, r.start + 30) + 30),
    ];
    inspect(r);
    draw();
  }
  function hostOptions() {
    const hosts = data.hosts.filter((h) =>
      data.runs.some((r) => r.host === h.tier && overlaps(r, scope())),
    );
    if (!hosts.some((h) => h.tier === state.host)) state.host = hosts[0]?.tier;
    el("host").replaceChildren();
    hosts.forEach((h) => el("host").add(new Option(hostName(h.tier), h.tier)));
    el("host").value = state.host;
  }
  function frame(id, height, left = 48) {
    const node = el(id),
      width = Math.max(280, node.clientWidth),
      right = 18,
      bottom = 30,
      top = 30;
    const svg = d3
      .select(node)
      .selectAll("svg")
      .data([null])
      .join("svg")
      .attr("viewBox", [0, 0, width, height])
      .attr("role", "img");
    svg.selectAll("*").remove();
    const x = d3
      .scaleUtc()
      .domain(domain().map((t) => new Date(t * 1000)))
      .range([left, width - right]);
    return { svg, x, width, height, left, right, top, bottom };
  }
  function axis(f, y) {
    f.svg
      .append("g")
      .attr("transform", "translate(0," + (f.height - f.bottom) + ")")
      .call(
        d3
          .axisBottom(f.x)
          .ticks(f.width < 500 ? 3 : 7)
          .tickFormat(time),
      );
    f.svg
      .append("g")
      .attr("transform", "translate(" + f.left + ",0)")
      .call(
        d3
          .axisLeft(y)
          .ticks(4)
          .tickSize(-(f.width - f.left - f.right)),
      )
      .select(".domain")
      .remove();
  }
  function selection(f) {
    if (!state.selected) return;
    const r = state.selected,
      d = domain(),
      a = Math.max(r.start, d[0]),
      b = Math.min(r.end, d[1]);
    if (b < a) return;
    f.svg
      .append("rect")
      .attr("x", f.x(new Date(a * 1000)))
      .attr("y", f.top)
      .attr(
        "width",
        Math.max(2, f.x(new Date(b * 1000)) - f.x(new Date(a * 1000))),
      )
      .attr("height", f.height - f.top - f.bottom)
      .attr("fill", colors[r.outcome])
      .attr("opacity", 0.09);
  }
  function milestone(f) {
    const t = data.cleanup,
      d = domain();
    if (!t || t < d[0] || t > d[1]) return;
    const x = f.x(new Date(t * 1000));
    f.svg
      .append("line")
      .attr("x1", x)
      .attr("x2", x)
      .attr("y1", f.top)
      .attr("y2", f.height - f.bottom)
      .attr("stroke", "#b7a3d2")
      .attr("stroke-dasharray", "3 4");
  }
  function timeline() {
    const hosts = data.hosts.filter((h) =>
      data.runs.some((r) => r.host === h.tier && overlaps(r, scope())),
    );
    const f = frame(
      "timeline",
      48 + hosts.length * 42,
      window.innerWidth < 650 ? 110 : 150,
    );
    f.svg.attr(
      "aria-label",
      "Test timeline. Green passed, amber pressure, red failed attempts, blue grey validation checks.",
    );
    const y = d3
      .scaleBand()
      .domain(hosts.map((h) => h.tier))
      .range([10, f.height - 30])
      .padding(0.28);
    f.svg
      .append("g")
      .attr("transform", "translate(" + (f.left - 8) + ",0)")
      .call(d3.axisLeft(y).tickSize(0).tickFormat(hostName))
      .select(".domain")
      .remove();
    f.svg
      .append("g")
      .attr("transform", "translate(0," + (f.height - 25) + ")")
      .call(
        d3
          .axisBottom(f.x)
          .ticks(f.width < 500 ? 3 : 8)
          .tickFormat(time),
      );
    const visible = data.runs.filter((r) => overlaps(r, domain()));
    const bars = f.svg
      .append("g")
      .selectAll("rect")
      .data(visible)
      .join("rect")
      .attr("class", "run")
      .attr("x", (r) => f.x(new Date(Math.max(r.start, domain()[0]) * 1000)))
      .attr("y", (r) => y(r.host))
      .attr("width", (r) =>
        Math.max(
          3,
          f.x(new Date(Math.min(r.end, domain()[1]) * 1000)) -
            f.x(new Date(Math.max(r.start, domain()[0]) * 1000)),
        ),
      )
      .attr("height", y.bandwidth())
      .attr("rx", 1)
      .attr("fill", (r) => colors[r.outcome])
      .attr("opacity", (r) =>
        state.game && !r.games.includes(state.game) ? 0.14 : 1,
      )
      .attr("stroke", (r) =>
        state.selected?.id === r.id ? "#fff" : r.end_known ? "none" : "#e5e9e3",
      )
      .attr("stroke-dasharray", (r) => (r.end_known ? null : "2 2"))
      .attr("tabindex", 0)
      .attr(
        "aria-label",
        (r) => describe(r) + ", " + outcomes[r.outcome] + ", " + stamp(r.start),
      )
      .on("pointermove", (event, r) =>
        tooltip(
          event,
          describe(r) +
            "\n" +
            outcomes[r.outcome] +
            "\n" +
            stamp(r.start) +
            " → " +
            (r.end_known ? stamp(r.end) : "finish not captured"),
        ),
      )
      .on("pointerleave", () => (tip.hidden = true))
      .on("click", (_, r) => {
        tip.hidden = true;
        choose(r);
      })
      .on("keydown", (event, r) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          choose(r);
        }
      });
    bars.append("title").text((r) => r.label);
    f.svg
      .append("g")
      .attr("pointer-events", "none")
      .selectAll("line")
      .data(
        visible.filter(
          (r) => r.m0 && r.m1 && r.m1 >= domain()[0] && r.m0 <= domain()[1],
        ),
      )
      .join("line")
      .attr("x1", (r) => f.x(new Date(Math.max(r.m0, domain()[0]) * 1000)))
      .attr("x2", (r) => f.x(new Date(Math.min(r.m1, domain()[1]) * 1000)))
      .attr("y1", (r) => y(r.host) + y.bandwidth() - 3)
      .attr("y2", (r) => y(r.host) + y.bandwidth() - 3)
      .attr("stroke", "#f1f5ed")
      .attr("opacity", (r) =>
        state.game && !r.games.includes(state.game) ? 0 : 0.65,
      )
      .attr("stroke-width", 2);
    if (
      data.cleanup &&
      data.cleanup >= domain()[0] &&
      data.cleanup <= domain()[1]
    ) {
      const x = f.x(new Date(data.cleanup * 1000));
      f.svg
        .append("line")
        .attr("x1", x)
        .attr("x2", x)
        .attr("y1", 2)
        .attr("y2", f.height - 30)
        .attr("stroke", "#b7a3d2")
        .attr("stroke-dasharray", "3 4");
      f.svg
        .append("text")
        .attr("x", Math.min(x + 5, f.width - 125))
        .attr("y", 9)
        .attr("fill", "#c7b3df")
        .attr("font-size", 10)
        .text("RAM cleanup recorded");
    }
  }
  function brush() {
    const f = frame("brush", 55, 48),
      s = scope();
    const x = d3
      .scaleUtc()
      .domain(s.map((t) => new Date(t * 1000)))
      .range([f.left, f.width - f.right]);
    f.svg
      .append("text")
      .attr("x", f.left)
      .attr("y", 10)
      .attr("fill", "#aab7ad")
      .text("Drag a window here");
    f.svg
      .append("g")
      .selectAll("rect")
      .data(data.runs.filter((r) => overlaps(r, s)))
      .join("rect")
      .attr("x", (r) => x(new Date(r.start * 1000)))
      .attr("y", 17)
      .attr("width", (r) =>
        Math.max(1, x(new Date(r.end * 1000)) - x(new Date(r.start * 1000))),
      )
      .attr("height", 21)
      .attr("fill", (r) => colors[r.outcome])
      .attr("opacity", 0.65);
    const control = d3
      .brushX()
      .extent([
        [f.left, 15],
        [f.width - f.right, 42],
      ])
      .on("end", (event) => {
        if (!event.sourceEvent) return;
        state.window = event.selection
          ? event.selection.map((p) => +x.invert(p) / 1000)
          : null;
        draw();
      });
    f.svg
      .append("g")
      .attr("class", "brush")
      .call(control)
      .call(
        control.move,
        domain().map((t) => x(new Date(t * 1000))),
      );
  }
  function segments(rows) {
    const parts = [];
    let part = [];
    for (const row of rows) {
      if (part.length && row[0] - part.at(-1)[0] > 10) {
        parts.push(part);
        part = [];
      }
      part.push(row);
    }
    if (part.length) parts.push(part);
    return parts;
  }
  function metric(
    id,
    title,
    rows,
    index,
    maxValue,
    color,
    unit,
    threshold,
    baseline,
  ) {
    const f = frame(id, 180),
      y = d3
        .scaleLinear()
        .domain([0, maxValue])
        .range([f.height - f.bottom, f.top]);
    f.svg.attr("aria-label", title + " over time");
    f.svg
      .append("text")
      .attr("class", "chart-title")
      .attr("x", f.left)
      .attr("y", 16)
      .text(title);
    axis(f, y);
    selection(f);
    milestone(f);
    if (threshold != null) {
      const badTop = id === "memory" ? y(threshold) : f.top;
      f.svg
        .append("rect")
        .attr("x", f.left)
        .attr("y", badTop)
        .attr("width", f.width - f.left - f.right)
        .attr(
          "height",
          id === "memory"
            ? f.height - f.bottom - y(threshold)
            : y(threshold) - f.top,
        )
        .attr("fill", "#ec897d")
        .attr("opacity", 0.07);
      f.svg
        .append("line")
        .attr("x1", f.left)
        .attr("x2", f.width - f.right)
        .attr("y1", y(threshold))
        .attr("y2", y(threshold))
        .attr("stroke", "#e3b45d")
        .attr("stroke-dasharray", "4 4");
    }
    if (baseline != null) {
      f.svg
        .append("line")
        .attr("x1", f.left)
        .attr("x2", f.width - f.right)
        .attr("y1", y(baseline))
        .attr("y2", y(baseline))
        .attr("stroke", "#8bcad5")
        .attr("stroke-dasharray", "2 4");
      f.svg
        .append("text")
        .attr("x", f.width - f.right - 4)
        .attr("y", y(baseline) - 4)
        .attr("text-anchor", "end")
        .attr("fill", "#8bcad5")
        .attr("font-size", 9)
        .text("credit baseline");
    }
    const visible = rows.filter(
      (r) => r[0] >= domain()[0] && r[0] <= domain()[1] && r[index] != null,
    );
    if (id === "generator") {
      f.svg
        .append("g")
        .selectAll("rect")
        .data(visible.filter((r) => r[2] != null && r[2] > 5))
        .join("rect")
        .attr("x", (r) => f.x(new Date(r[0] * 1000)))
        .attr("width", (r) =>
          Math.max(
            1,
            f.x(new Date((r[0] + 2) * 1000)) - f.x(new Date(r[0] * 1000)),
          ),
        )
        .attr("y", f.top)
        .attr("height", f.height - f.top - f.bottom)
        .attr("fill", "#ec897d")
        .attr("opacity", 0.18);
    }
    const line = d3
      .line()
      .x((r) => f.x(new Date(r[0] * 1000)))
      .y((r) => y(r[index]));
    if (id === "density") line.curve(d3.curveStepAfter);
    f.svg
      .append("g")
      .selectAll("path")
      .data(segments(visible))
      .join("path")
      .attr("d", line)
      .attr("fill", "none")
      .attr("stroke", color)
      .attr("stroke-width", 1.7);
    if (!visible.length)
      f.svg
        .append("text")
        .attr("x", f.left + 12)
        .attr("y", f.top + 35)
        .attr("fill", "#aab7ad")
        .text("No samples in this window");
    const cursor = f.svg
      .append("line")
      .attr("y1", f.top)
      .attr("y2", f.height - f.bottom)
      .attr("stroke", "#dbe5df")
      .attr("stroke-dasharray", "2 3")
      .style("display", "none");
    const label = f.svg
      .append("text")
      .attr("class", "hover-label")
      .attr("x", f.width - f.right)
      .attr("text-anchor", "end")
      .attr("y", 16);
    f.svg
      .append("rect")
      .attr("x", f.left)
      .attr("y", f.top)
      .attr("width", f.width - f.left - f.right)
      .attr("height", f.height - f.top - f.bottom)
      .attr("fill", "transparent")
      .on("pointermove", (event) => {
        if (!visible.length) return;
        const seconds = +f.x.invert(d3.pointer(event)[0]) / 1000;
        const i = d3.bisector((r) => r[0]).center(visible, seconds),
          r = visible[i];
        if (Math.abs(r[0] - seconds) > 10) {
          cursor.style("display", "none");
          label.text("No sample");
          return;
        }
        const px = f.x(new Date(r[0] * 1000));
        cursor.attr("x1", px).attr("x2", px).style("display", null);
        label.text(
          time(new Date(r[0] * 1000)) +
            " · " +
            format(r[index], id === "density" ? 0 : 1) +
            " " +
            unit,
        );
      })
      .on("pointerleave", () => {
        cursor.style("display", "none");
        label.text("");
      });
  }
  function draw() {
    timeline();
    brush();
    const h = data.hosts.find((h) => h.tier === state.host),
      raw = samples.get(state.host) || [];
    const rows = raw.map((r) => [r[1], r[2], r[3] * 100, r[4]]);
    metric(
      "cpu",
      "Host CPU · cores",
      rows,
      1,
      h?.vcpus || 2,
      "#8bcad5",
      "cores",
      (h?.vcpus || 2) * (1 - data.policy.cpu_headroom_fraction),
      h?.baseline_cores,
    );
    metric(
      "memory",
      "Host free RAM · %",
      rows,
      2,
      100,
      "#9dcc8a",
      "%",
      data.policy.memory_headroom_fraction * 100,
    );
    metric(
      "density",
      "Running game servers",
      rows,
      3,
      Math.max(1, d3.max(rows, (r) => r[3]) || 0),
      "#c6cbd4",
      "servers",
    );
    metric(
      "generator",
      "Local free RAM · GiB",
      data.generator,
      1,
      Math.max(
        4,
        Math.ceil((d3.max(data.generator, (r) => r[1]) || 4) / 4) * 4,
      ),
      "#b7a3d2",
      "GiB",
    );
  }
  el("session").onchange = () => {
    state.session =
      el("session").value === "all" ? "all" : Number(el("session").value);
    state.window = null;
    state.selected = null;
    el("inspector").replaceChildren(
      ...emptyInspector.cloneNode(true).childNodes,
    );
    el("inspector").style.borderColor = "";
    hostOptions();
    draw();
  };
  el("host").onchange = () => {
    state.host = el("host").value;
    draw();
  };
  el("game").onchange = () => {
    state.game = el("game").value;
    draw();
  };
  el("reset").onclick = () => {
    state.window = null;
    draw();
  };
  let previousWidth = 0;
  new ResizeObserver((entries) => {
    const width = Math.round(entries[0].contentRect.width);
    if (width !== previousWidth) {
      previousWidth = width;
      draw();
    }
  }).observe(el("timeline"));
  hostOptions();
  draw();
})();
