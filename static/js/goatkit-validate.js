// GoatKit Form Validation - minimal client-side required field & pattern enforcement
(function(){
 if(window.GoatKitValidateLoaded) return; window.GoatKitValidateLoaded=true;
 const SEL_FORM='form[data-gk-validate]';
 const ATTR_ERROR='data-gk-error';
 const CLASS_ERROR='gk-field-error';
 const EVENT_BLOCK='gk:validation:block';
 const EVENT_OK='gk:validation:ok';
 function $(sel,ctx=document){return ctx.querySelector(sel);} function $all(sel,ctx=document){return Array.from(ctx.querySelectorAll(sel));}
 function fieldLabel(input){
  const id=input.getAttribute('id'); if(id){ const lbl=document.querySelector(`label[for="${id}"]`); if(lbl) return lbl.textContent.trim().replace(/[*:]/g,''); }
  // fallback: name attr
  return (input.name||'Field');
 }
 function createErrorEl(msg){ const div=document.createElement('div'); div.setAttribute(ATTR_ERROR,''); div.className='mt-1 text-xs text-red-600 dark:text-red-400'; div.textContent=msg; return div; }
 function clearErrors(form){ $all(`[${ATTR_ERROR}]`,form).forEach(el=>el.remove()); $all('.'+CLASS_ERROR,form).forEach(el=>{ el.classList.remove(CLASS_ERROR); el.removeAttribute('aria-invalid'); }); }
 function report(form, failures){ clearErrors(form); if(!failures.length){ form.dispatchEvent(new CustomEvent(EVENT_OK,{bubbles:true})); return true; }
  failures.forEach(f=>{ const {input,msg} = f; const parent=input.closest('.mt-2')||input.parentElement||input; if(parent && !parent.querySelector(`[${ATTR_ERROR}]`)){ parent.appendChild(createErrorEl(msg)); }
   input.classList.add(CLASS_ERROR); input.setAttribute('aria-invalid','true'); input.setAttribute('aria-describedby', (input.getAttribute('aria-describedby')||'') + ' validation-error'); });
  form.dispatchEvent(new CustomEvent(EVENT_BLOCK,{detail:{failures},bubbles:true}));
  return false;
 }
 function validateField(input){
  if(input.disabled) return null;
  const v=(input.value||'').trim();
  if(input.hasAttribute('required') && !v){ return {input, msg: fieldLabel(input)+' is required'}; }
  if(v && input.dataset.minLength){ const m=parseInt(input.dataset.minLength,10); if(!isNaN(m) && v.length<m){ return {input,msg: fieldLabel(input)+` must be at least ${m} characters`}; }}
  if(v && input.dataset.pattern){ try { const re=new RegExp(input.dataset.pattern); if(!re.test(v)){ return {input,msg: fieldLabel(input)+' has invalid format'}; } }catch(_){/* ignore invalid pattern */} }
  if(input.type==='file' && input.files){ const max=parseInt(input.dataset.maxFileSize||'',10); if(!isNaN(max)){ for(const f of input.files){ if(f.size>max){ return {input,msg: `${f.name} exceeds maximum size ${(max/1024/1024).toFixed(1)}MB`}; } } } }
  return null;
 }
 function gatherInputs(form){ return $all('input,textarea,select',form).filter(i=>!i.closest('[data-gk-ignore]')); }
 function validateForm(form){ const failures=[]; gatherInputs(form).forEach(inp=>{ const r=validateField(inp); if(r) failures.push(r); }); return report(form,failures); }
function attach(form){ if(form.__gkValidateAttached) return; form.__gkValidateAttached=true; 
 function maybeSummary(failures){ const box=form.querySelector('#form-messages'); if(!box) return; if(!failures||!failures.length){ box.innerHTML=''; return; } const list=failures.map(f=>`<li>${fieldLabel(f.input)}: ${f.msg.replace(/^.*? is required$/,'required')}</li>`).join(''); box.innerHTML=`<div class="rounded-md bg-red-50 dark:bg-red-900/20 p-3 mb-4"><p class="text-sm font-medium text-red-700 dark:text-red-200 mb-1">Please correct the following:</p><ul class="list-disc list-inside text-xs space-y-0.5 text-red-700 dark:text-red-300">${list}</ul></div>`; }
 form.addEventListener('gk:validation:block',e=>maybeSummary(e.detail.failures));
 form.addEventListener('gk:validation:ok',()=>maybeSummary([]));
 form.addEventListener('submit',function(e){ if(!validateForm(form)){ e.preventDefault(); e.stopPropagation(); } });
 form.addEventListener('htmx:beforeRequest',function(e){ if(e.target===form && !validateForm(form)){ e.preventDefault(); e.stopPropagation(); } });
 form.addEventListener('input',function(e){ const t=e.target; if(!(t instanceof HTMLElement)) return; if(t.matches('input,textarea,select')){ const err=validateField(t); if(!err){ const wrap=t.closest('.mt-2')||t.parentElement; if(wrap){ const existing=wrap.querySelector(`[${ATTR_ERROR}]`); if(existing){ existing.remove(); t.classList.remove(CLASS_ERROR); t.removeAttribute('aria-invalid'); } } } }});
}
 function init(){ $all(SEL_FORM).forEach(attach); }
 document.addEventListener('DOMContentLoaded',init); new MutationObserver(init).observe(document.documentElement,{subtree:true,childList:true});
 // styles
 const style=document.createElement('style'); style.textContent=`.${CLASS_ERROR}{outline:1px solid #dc2626 !important; box-shadow:0 0 0 1px #dc2626 inset}`; document.head.appendChild(style);
 window.GoatKitValidate={validateForm};
})();
