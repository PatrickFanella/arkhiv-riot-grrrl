import { getDocument, GlobalWorkerOptions } from 'pdfjs-dist';
import worker from 'pdfjs-dist/build/pdf.worker.min.mjs?url';
import type { PrintBackend } from '@subcult/archive-viewer/print';

GlobalWorkerOptions.workerSrc = worker;
export const pdfBackend: PrintBackend = {
  open(url) {
    const task = getDocument({url,canvasMaxAreaInBytes:32*1024*1024,isEvalSupported:false,enableXfa:false,cMapUrl:'/pdfjs/cmaps/',cMapPacked:true,standardFontDataUrl:'/pdfjs/standard_fonts/',wasmUrl:'/pdfjs/wasm/'});
    return {destroy: () => task.destroy(),promise: task.promise.then(pdf => ({
      numPages:pdf.numPages,
      async page(number) {
        const page = await pdf.getPage(number);
        const dimensions = page.getViewport({scale:1});
        return {width:dimensions.width,height:dimensions.height,
          render(canvas,scale) {
            const viewport = page.getViewport({scale});
            canvas.width = Math.ceil(viewport.width); canvas.height = Math.ceil(viewport.height);
            const context = canvas.getContext('2d');
            if (!context) throw new Error('Canvas is unavailable');
            const render = page.render({canvas,canvasContext:context,viewport});
            return {promise:render.promise,cancel:()=>render.cancel()};
          },
          async text() { const content = await page.getTextContent(); return content.items.map(item => 'str' in item ? item.str + (item.hasEOL ? '\n' : ' ') : '').join(''); }
        };
      }
    }))};
  }
};
