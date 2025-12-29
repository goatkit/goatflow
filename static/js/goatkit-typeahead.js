// Generic typeahead enhancement: Enter/Tab commits first suggestion when none highlighted
// Applies to inputs matching any of the attribute selectors below. Minimal invasive.
(function(){
	if(window.GoatKitTypeaheadLoaded) return; // idempotent
	window.GoatKitTypeaheadLoaded=true;
	const SELECTORS=[
		'input[data-typeahead]','input[data-lookup]','input[data-autocomplete]','input[list]' // datalist fallback
	];
	function isTypeaheadInput(el){
		return el instanceof HTMLInputElement && SELECTORS.some(sel=>el.matches(sel));
	}
	function getSuggestionList(el){
		// ARIA pattern
		const ctrl=el.getAttribute('aria-controls');
		if(ctrl){
			const list=document.getElementById(ctrl); if(list) return list;
		}
		// sibling listbox or ul
		let n=el.nextElementSibling;
		if(n && matchesList(n)) return n;
		// datalist (native) – treat options as suggestions
		if(el.list) return el.list;
		return null;
	}
	function matchesList(node){
		if(!node) return false;
		if(node.getAttribute && node.getAttribute('role')==='listbox') return true;
		if(node.tagName==='UL' || node.tagName==='OL') return true;
		if(node.classList && (node.classList.contains('typeahead-list')||node.classList.contains('suggestions'))) return true;
		return false;
	}
	function isVisible(el){
		if(!el) return false;
		if(el.hidden) return false;
		const s=window.getComputedStyle(el);
		return s.display!=='none' && s.visibility!=='hidden';
	}
	function firstItem(list){
		if(!list) return null;
		// active item first
		let active=list.querySelector('.active,[aria-selected="true"],[data-active="true"]');
		if(active) return active;
		// listbox pattern
		let options=list.querySelectorAll('[role="option"]');
		if(options.length) return options[0];
		// generic li children
		options=list.querySelectorAll('li');
		if(options.length) return options[0];
		// datalist option elements
		if(list.tagName==='DATALIST'){
			const opts=list.querySelectorAll('option');
			if(opts.length) return opts[0];
		}
		return null;
	}
	function extractValue(item){
		if(!item) return '';
		if(item.tagName==='OPTION') return item.value || item.getAttribute('value') || item.textContent.trim();
		return item.getAttribute('data-value') || item.getAttribute('data-id') || item.textContent.trim();
	}
	function markActive(item,list){
		if(!item||!list) return;
		list.querySelectorAll('.gk-ta-active').forEach(el=>el.classList.remove('gk-ta-active'));
		item.classList.add('gk-ta-active');
	}
	function commit(input,item){
		const canonical=extractValue(item);
		if(!canonical) return false;
		let displayVal=canonical;
		// For GoatKit autocomplete enhanced inputs, prefer the rendered display (textContent) over canonical login
		if(input.dataset && input.dataset.gkAutocomplete){
			const txt=(item.textContent||'').trim();
			if(txt) displayVal=txt; // textContent holds compiled template
		}
		const changed = input.value!==displayVal;
		input.value=displayVal;
		// Hidden target support (data-hidden-target points to hidden input ID)
		if(input.dataset.hiddenTarget){
			const hidden=document.getElementById(input.dataset.hiddenTarget);
			if(hidden){
				// Prefer explicit data-login/data-id value from item for hidden field, fallback to canonical
				const hiddenVal=item.getAttribute('data-login')||item.getAttribute('data-id')||canonical;
				if(hidden.value!==hiddenVal){
					hidden.value=hiddenVal;
					hidden.dispatchEvent(new Event('input',{bubbles:true}));
					hidden.dispatchEvent(new Event('change',{bubbles:true}));
				}
			}
		}
		if(changed){
			input.dispatchEvent(new Event('input',{bubbles:true}));
			input.dispatchEvent(new Event('change',{bubbles:true}));
			// brief visual flash to show commit
			input.classList.add('gk-ta-committed');
			setTimeout(()=>input.classList.remove('gk-ta-committed'),200);
		}
		return true;
	}
	function focusNext(input){
		if(!input.form) return;
		const focusables=Array.from(input.form.querySelectorAll('input,select,textarea,button,[tabindex]:not([tabindex="-1"])'))
			.filter(el=>!el.disabled && el.offsetParent!==null);
		const idx=focusables.indexOf(input);
		if(idx>-1 && idx+1<focusables.length) focusables[idx+1].focus();
	}
		document.addEventListener('keydown',function(e){
		if(!(e.key==='Enter'||e.key==='Tab')) return;
		const target=e.target;
		if(!isTypeaheadInput(target)) return;
		const list=getSuggestionList(target);
			if(!isVisible(list)) return;
		const item=firstItem(list);
		if(!item) return;
			markActive(item,list);
		const changed=commit(target,item);
			if(!changed) return; // still prevent submit if commit logic set hidden field
		// Always hide list after auto commit
		if(list){ list.classList.add('hidden'); }
			if(e.key==='Enter'){
				// Always prevent form submit when auto-completing
				e.preventDefault();
				focusNext(target);
			}else{
				// Tab: let default advance
			}
	},true); // capture to preempt default submit

	// Auto-highlight first suggestion when lists become visible (MutationObserver)
	const observer=new MutationObserver(muts=>{
		for(const m of muts){
			if(m.type==='attributes' && m.attributeName==='class'){
				const node=m.target;
				if(matchesList(node) && !node.classList.contains('hidden') && isVisible(node)){
					const fi=firstItem(node);
					if(fi) markActive(fi,node);
				}
			}
		}
	});
	// Observe suggestion containers added later (wait for body)
	(function attachTAObserver(){
		if(!document.body){
			return setTimeout(attachTAObserver,30);
		}
		observer.observe(document.body,{subtree:true,attributes:true,attributeFilter:['class']});
	})();

	// Inject minimal styles once
	const style=document.createElement('style');
	style.textContent=`.gk-ta-active{background:#2563eb;color:#fff !important}.dark .gk-ta-active{background:#1d4ed8}.gk-ta-committed{outline:2px solid #2563eb;}`;
	document.head.appendChild(style);

	// Customer info panel integration (specific enhancement; no dependency)
	function loadCustomerInfo(login){
		if(!login) return;
		const panel=document.getElementById('customer-info-panel');
		fetch(`/tickets/customer-info/${encodeURIComponent(login)}`,{headers:{'HX-Request':'true'}})
			.then(r=>{if(!r.ok) throw r.status; return r.text();})
			.then(html=>{ if(panel){ panel.innerHTML=html; panel.classList.remove('hidden'); } })
			.catch(err=>{ if(window.GK_DEBUG) console.warn('customer-info fetch failed',err); });
	}

	function bindCustomerUserEvents(){
		const hidden=document.getElementById('customer_user_id');
		const input=document.getElementById('customer_user_input');
		if(!input) return;
		if(hidden && !hidden.dataset.gkBound){
			hidden.addEventListener('change',()=>{ if(window.GK_DEBUG) console.log('customer_user_id change ->',hidden.value); loadCustomerInfo(hidden.value); });
			hidden.dataset.gkBound='1';
		}
		if(!input.dataset.gkCustBound){
			input.addEventListener('blur',()=>{
				const val=input.value.trim();
				if(val && /.+@.+\..+/.test(val)){
					// Prefer explicit email typed even if hidden login already populated
					if(window.GK_DEBUG){ console.log('customer-info blur prefers email', val, 'over hidden', hidden && hidden.value); }
					loadCustomerInfo(val);
					return;
				}
				// Fallback: no email typed; if no hidden value yet, try raw value when it looks like an identifier
				const hiddenVal=hidden?hidden.value:"";
				if(!hiddenVal && val){
					loadCustomerInfo(val);
				}
			});
			// If user edits after commit and adds '@', trigger lookup quickly (debounced minimal)
			let emailDebounce;
			input.addEventListener('input',()=>{
				const v=input.value.trim();
				if(/.+@.+\..+/.test(v)){
					clearTimeout(emailDebounce);
					emailDebounce=setTimeout(()=>{ if(window.GK_DEBUG) console.log('customer-info live email detect', v); loadCustomerInfo(v); },350);
				}
			});
			// Listen for custom event when a suggestion is committed (emitted in commit logic)
			input.addEventListener('gk:typeahead:commit',e=>{ const v=e.detail && e.detail.value; if(v){ if(window.GK_DEBUG) console.log('typeahead commit ->',v); loadCustomerInfo(v); }});
			input.dataset.gkCustBound='1';
		}
	}

	// Attempt immediate bind
	bindCustomerUserEvents();
	// Re-bind after DOMContentLoaded just in case template loaded later
	document.addEventListener('DOMContentLoaded',bindCustomerUserEvents);
})();
