import { chromium, expect, test } from "@playwright/test";

const blocks = Array.from({ length: 32 }, (_, i) => String.fromCodePoint(0x2580 + i)).join("");
const matrix = "█▀▄▌▐▖▗▘▝▚▞██";
const masks = [15, 3, 12, 5, 10, 4, 8, 1, 2, 9, 6, 15, 15];

for (const dpr of [1, 2]) {
  for (const fallback of [false, true]) {
    test(`block cells and pixels at DPR ${dpr}, ${fallback ? "fallback" : "default"}`, async ({}, testInfo) => {
      // The browser scale keeps device-pixel ResizeObserver sizes aligned with DPR.
      const browser = await chromium.launch({ args: [`--force-device-scale-factor=${dpr}`] });
      try {
        const context = await browser.newContext({ deviceScaleFactor: dpr });
        const page = await context.newPage();
        await page.goto(`/tests/terminal-rendering.html${fallback ? "?fallback" : ""}`);
        await page.waitForFunction(() => window.fixture);
        const sample = `\x1b[?25l\x1b[37;40m${blocks}\r\n\x1b[1m${blocks}\r\n\x1b[0;37;40m${matrix}\r\n\x1b[1m${matrix}\r\n\x1b[0;37;40mASCII abc 0123 \ue0b0`;
        await page.evaluate((sample) => { fixture.create(); fixture.send(0, sample); fixture.done(0); }, sample);
        await page.waitForFunction(() => fixture.terminals[0].completed === 1);
        const initial = await page.evaluate(() => fixture.lines(0));
        await page.evaluate((sample) => {
          fixture.send(0, "", true);
          const bytes = [...new TextEncoder().encode(sample)];
          const split = bytes.indexOf(0xe2) + 1;
          fixture.bytes(0, bytes.slice(0, split));
          fixture.bytes(0, bytes.slice(split));
          fixture.done(0);
        }, sample);
        await page.waitForFunction(() => fixture.terminals[0].completed === 2);
        expect(await page.evaluate(() => fixture.lines(0))).toEqual(initial);
        await page.evaluate((sample) => { fixture.send(0, sample, true); fixture.send(0, " live"); fixture.done(0); }, sample);
        await page.waitForFunction(() => fixture.terminals[0].completed === 3);
        await page.evaluate(() => { fixture.terminals[0].host.style.width = "760px"; fixture.terminals[0].fit.fit(); });
        const geometry = await page.evaluate(() => {
          const term = fixture.terminals[0].term;
          const service = term._core._renderService;
          const renderer = service._renderer.value;
          return { cell: service.dimensions.css.cell, renderer: renderer.constructor.name, lost: renderer._gl?.isContextLost(),
            webgl: !!renderer._gl, canvasWidth: renderer._canvas?.width, fontSize: term.options.fontSize,
            widths: [0, 1].map((row) => Array.from({ length: 32 }, (_, col) => term.buffer.active.getLine(row).getCell(col).getWidth())) };
        });
        expect(geometry.webgl).toBe(!fallback);
        expect(geometry.fontSize).toBe(13);
        expect(geometry.widths.flat()).toEqual(Array(64).fill(1));
        expect(await page.evaluate(() => fixture.lines(0).slice(0, 2))).toEqual([blocks, blocks]);
        expect(await page.evaluate(() => fixture.lines(0)[4])).toBe("ASCII abc 0123 \ue0b0 live");
        await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        const screenshot = await page.locator(".xterm-screen").screenshot({ path: testInfo.outputPath("blocks.png") });
        await testInfo.attach("block-matrix", { body: screenshot, contentType: "image/png" });
        await testInfo.attach("renderer", { body: JSON.stringify({ dpr, fallback, ...geometry }), contentType: "application/json" });
        const pixels = await page.evaluate(async ({ png, cell, dpr, masks }) => {
          const image = new Image();
          image.src = "data:image/png;base64," + png;
          await image.decode();
          const canvas = document.createElement("canvas");
          canvas.width = image.width; canvas.height = image.height;
          const ctx = canvas.getContext("2d");
          ctx.drawImage(image, 0, 0);
          const results = [];
          for (const row of [2, 3]) {
            for (let col = 0; col < masks.length; col++) {
              for (let quadrant = 0; quadrant < 4; quadrant++) {
                const x = Math.floor((col + (quadrant % 2 ? 0.75 : 0.25)) * cell.width * dpr);
                const y = Math.floor((row + (quadrant > 1 ? 0.75 : 0.25)) * cell.height * dpr);
                const rgb = [...ctx.getImageData(x, y, 1, 1).data].slice(0, 3);
                results.push({ row, col, quadrant, expected: !!(masks[col] & (1 << quadrant)), filled: Math.max(...rgb) > 100 });
              }
            }
          }
          for (const row of [2, 3]) {
            for (const offset of [-1, 0, 1]) {
              const x = Math.floor(12 * cell.width * dpr) + offset;
              const y = Math.floor((row + 0.5) * cell.height * dpr);
              const rgb = [...ctx.getImageData(x, y, 1, 1).data].slice(0, 3);
              results.push({ row, edge: offset, expected: true, filled: Math.max(...rgb) > 100 });
            }
          }
          return results;
        }, { png: screenshot.toString("base64"), cell: geometry.cell, dpr, masks });
        expect(pixels.filter((pixel) => pixel.expected !== pixel.filled)).toEqual([]);
      } finally {
        await browser.close();
      }
    });
  }
}
