// SPDX-License-Identifier: AGPL-3.0-or-later
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const jsQR = require("../web/jsqr.js");
const scanner = require("../web/esim-qr.js");
const fixtures = require("./fixtures/esim-qr.json");

test("decode a real QR image and accept only eSIM activation data", () => {
  for (const fixture of fixtures) {
    for (const inverted of [false, true]) {
      const cell = 7, quiet = 4, size = (fixture.modules.length + quiet * 2) * cell;
      const pixels = new Uint8ClampedArray(size * size * 4);
      for (let y = 0; y < size; y++) for (let x = 0; x < size; x++) {
        const row = Math.floor(y / cell) - quiet, col = Math.floor(x / cell) - quiet;
        const dark = fixture.modules[row] && fixture.modules[row][col] === "1";
        const value = (inverted ? !dark : dark) ? 0 : 255;
        const offset = (y * size + x) * 4;
        pixels[offset] = pixels[offset + 1] = pixels[offset + 2] = value;
        pixels[offset + 3] = 255;
      }
      const result = jsQR(pixels, size, size, { inversionAttempts: "attemptBoth" });
      assert.equal(result.data, fixture.data);
      assert.equal(scanner.activationCode(result.data), fixture.activation ? fixture.data : null);
    }
  }
});

test("reject URLs, malformed codes and control characters without following links", () => {
  const valid = "LPA:1$smdp.example.invalid$DEMO-NOT-A-REAL-ESIM";
  assert.equal(scanner.activationCode("  " + valid + "\n"), valid);
  assert.equal(scanner.activationCode(valid.slice(4)), valid);
  assert.equal(scanner.activationCode("LPA:1$smdp.example.invalid:443$demo"), "LPA:1$smdp.example.invalid:443$demo");
  for (const value of ["https://example.invalid/", "javascript:alert(1)", "LPA:1$$demo", "LPA:2$smdp.example.invalid$demo", "LPA:1$host/path$demo", "LPA:1$user@host$demo", "LPA:1$host$de\x00mo", "LPA:1$host$de\nmo", valid + "x".repeat(1024)]) {
    assert.equal(scanner.activationCode(value), null);
  }
});

test("close while camera permission is pending releases the eventual stream", async () => {
  const nodes = new Map();
  const document = {
    activeElement: null,
    getElementById(id) {
      if (!nodes.has(id)) nodes.set(id, { hidden: true, value: "", isConnected: true, disabled: false, addEventListener() {}, focus() { document.activeElement = this; }, pause() {} });
      return nodes.get(id);
    },
    createElement() { return {}; },
    addEventListener() {},
    head: { appendChild() { throw new Error("Decoder must not load after cancellation"); } }
  };
  let grant, stopped = 0, accepted = 0;
  const navigator = { mediaDevices: { getUserMedia: () => new Promise(resolve => { grant = resolve; }) } };
  const window = { isSecureContext: true, addEventListener() {}, cancelAnimationFrame() {} };
  const context = { document, navigator, window, module: { exports: {} }, URL, Promise };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, "../web/esim-qr.js"), "utf8"), context);
  const instance = context.module.exports.create({ t: key => key, onResult: () => accepted++ });
  instance.open();
  instance.startCamera();
  instance.close();
  grant({ getTracks: () => [{ stop() { stopped++; } }] });
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(stopped, 1);
  assert.equal(accepted, 0);
  assert.equal(document.getElementById("euicc-qr-modal").hidden, true);
});
