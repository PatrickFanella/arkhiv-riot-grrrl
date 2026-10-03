// Scheduled recovery proof against a fresh, isolated restored site.
const { chromium } = require('@playwright/test');
const { createHash } = require('node:crypto');
const fs = require('node:fs');

async function main() {
  const origin = process.env.PROBE_ORIGIN || 'http://web';
  const expected = JSON.parse(fs.readFileSync('/proof/expected.json', 'utf8'));
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  let sourceRequests = 0;
  await context.route(/https?:\/\/(?:[^/]+\.)?archive\.org\//, route => { sourceRequests++; return route.abort(); });
  const page = await context.newPage();
  const get = async path => {
    const response = await context.request.get(origin + path);
    if (!response.ok()) throw new Error('restored request failed: ' + response.status());
    return response;
  };
  try {
    let ready = false;
    for (let attempt = 0; attempt < 30; attempt++) {
      try { await get('/api/ready'); ready = true; break; } catch { await new Promise(resolve => setTimeout(resolve, 1000)); }
    }
    if (!ready) throw new Error('restored API did not become ready');
    const documents = [];
    for (let offset = 0; ; offset += 50) {
      const result = await (await get('/api/documents?limit=50&offset=' + offset)).json();
      documents.push(...result.documents);
      if (documents.length >= result.total) break;
      if (!result.documents.length || offset > 1000) throw new Error('invalid catalog pagination');
    }
    if (documents.length !== expected.published) throw new Error('published count differs from restored catalog');
    if (expected.variants) {
      const actual = documents.flatMap(document => document.variants.map(variant => ({...variant, document: document.id})));
      if (actual.length !== expected.variants.length) throw new Error('reviewed variant count differs');
      for (const variant of actual) {
        const reviewed = expected.variants.find(v => v.id === variant.id);
        if (!reviewed || reviewed.document !== variant.document || reviewed.sha256 !== variant.sha256 || reviewed.size !== variant.size || (expected.public && !variant.public_approved)) throw new Error('public variant differs from exact review');
      }
    }
    let downloads = 0, renders = 0;
    const renderedCollections = new Set();
    for (const document of documents) {
      for (const variant of document.variants) {
        const response = await get('/api/assets/' + variant.id + '?download=1');
        const data = await response.body();
        if (data.length !== variant.size || createHash('sha256').update(data).digest('hex') !== variant.sha256) throw new Error('download integrity mismatch');
        await response.dispose();
        downloads++;
        await page.goto(origin + '/documents/' + document.id + '?read=1&variant=' + variant.id + '&page=1');
        await page.waitForFunction(() => {
          const c = document.querySelector('canvas');
          return c && c.width > 0 && c.height > 0 && !document.querySelector('.reader [role="status"]') && !document.querySelector('.reader [role="alert"]');
        }, null, { timeout: 30000 });
        const painted = await page.locator('canvas').evaluate(c => c.getContext('2d').getImageData(0, 0, c.width, c.height).data.some((v, i) => i % 4 !== 3 && v < 230));
        if (!painted) throw new Error('PDF canvas has no visible content');
        renders++;
        if (!renderedCollections.has(document.collection)) {
          await page.screenshot({ path: '/proof/' + document.collection + '-reader.png', fullPage: true });
          renderedCollections.add(document.collection);
        }
      }
    }
    for (const query of expected.queries || []) {
      const result = await (await get('/api/documents?collection=' + encodeURIComponent(query.collection) + '&q=' + encodeURIComponent(query.query))).json();
      if (result.total < query.minimum) throw new Error('restored search failed');
    }
    for (const path of expected.denied || []) {
      for (const method of ['GET','HEAD']) {
        const response = await context.request.fetch(origin + path, {method, headers:{'If-None-Match':'*', Range:'bytes=0-4'}});
        if (response.status() !== 404 || response.headers()['cache-control'] !== 'no-store') throw new Error('unapproved route reachable: '+path);
      }
    }
    if (expected.narrow && documents.length) {
      await page.setViewportSize({width:390,height:844});
      const document = documents[0], variant=document.variants[0];
      await page.goto(origin+'/documents/'+document.id+'?read=1&variant='+variant.id+'&page=1');
      await page.getByTestId('pdf-canvas').waitFor({state:'visible'});
      await page.waitForFunction(()=> !document.querySelector('.reader [role="status"]') && !document.querySelector('.reader [role="alert"]'));
      if (variant.pages>1) {
        await page.getByRole('button',{name:'Next page',exact:true}).click();
        await page.waitForFunction(()=>document.querySelector('input[aria-label="Page number"]').value==='2' && !document.querySelector('.reader [role="status"]'));
      }
      await page.getByLabel('Zoom',{exact:true}).selectOption('1.25');
      await page.waitForFunction(()=> !document.querySelector('.reader [role="status"]'));
      if (!(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))) throw new Error('narrow document overflows');
      await page.screenshot({path:'/proof/narrow-reader.png',fullPage:true});
      await page.goto(origin+'/?collection='+encodeURIComponent(document.collection));
      await page.getByLabel('Search titles and available text').waitFor();
      await page.getByLabel('Search titles and available text').fill(expected.queries?.[0]?.query || document.metadata.title);
      await page.getByRole('button',{name:'Search',exact:true}).click();
      await page.getByRole('status').filter({hasText:'documents'}).waitFor();
      if (!(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))) throw new Error('narrow search overflows');
      await page.screenshot({path:'/proof/narrow-search.png',fullPage:true});
    }
    const blocked = await page.evaluate(async () => { try { await fetch('https://archive.org/metadata/ZineArchive'); return false; } catch { return true; } });
    if (!blocked) throw new Error('source access was not blocked');
    const result = { documents: documents.length, downloads, renders, sourceBlocked: blocked, sourceRequests, collections: [...renderedCollections], deniedRoutes:(expected.denied||[]).length, narrow:!!expected.narrow };
    fs.writeFileSync('/proof/reader.json', JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result));
  } finally { await browser.close(); }
}
main().catch(error => { console.error(error.message); process.exitCode = 1; });
