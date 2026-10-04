import { PrintReader, type PrintHit } from '@subcult/archive-viewer/print';
import type { Variant } from './App';
import { pdfBackend } from './pdfBackend';

// Arkhiv owns asset routes, publication checks and document page links.
// Reader controls and cancellation come from the shared package.
export default function Reader({variant,initialPage}: {variant:Variant;initialPage:number}) {
  const id=encodeURIComponent(variant.id);
  const searchable=variant.text_status==='page-text'||variant.text_status==='document-text';
  return <PrintReader url={'/api/assets/'+id} title={variant.filename}
    initialPage={initialPage} backend={pdfBackend}
    onPageChange={page=>{const params=new URLSearchParams(location.search);params.set('page',String(page));history.replaceState(null,'','?'+params);}}
    search={searchable ? async (query,signal):Promise<PrintHit[]>=>{
      const response=await fetch('/api/assets/'+id+'/search?'+new URLSearchParams({q:query}),{signal,cache:'no-store'});
      if(!response.ok)throw new Error('Document search unavailable');
      return response.json();
    }:undefined}/>;
}
