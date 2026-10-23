import { nextTick, onBeforeUnmount, watch } from 'vue'

// Move focus into an opened drawer, contain keyboard navigation, and return it
// to the triggering control. The drawer owns its close callback.
export function useDialogFocus(open, element, close) {
 let returnTo = null
 const controls = () => [...(element.value?.querySelectorAll('button,input,select,textarea,summary,a[href],[tabindex]') || [])].filter(node => {
  if (node.disabled || node.tabIndex < 0 || node.closest('[hidden]')) return false
  const collapsed = node.closest('details:not([open])')
  return !collapsed || node.tagName === 'SUMMARY' && node.parentElement === collapsed
 })
 function keydown(event) {
  if (event.defaultPrevented || !open.value || !element.value) return
  const activeDialog = document.activeElement?.closest('[role="dialog"]')
  if (activeDialog && activeDialog !== element.value) return
  if (event.key === 'Escape') { event.preventDefault(); close(); return }
  if (event.key !== 'Tab') return
  const nodes=controls(), first=nodes[0], last=nodes[nodes.length-1]
  if (!first) {event.preventDefault();element.value.focus();return}
  if(event.shiftKey && (document.activeElement===first || !element.value.contains(document.activeElement))) {event.preventDefault();last.focus()}
  else if(!event.shiftKey && (document.activeElement===last || !element.value.contains(document.activeElement))) {event.preventDefault();first.focus()}
 }
 watch(open,async value=>{
  if(value){returnTo=document.activeElement;await nextTick();if(open.value){(controls()[0] || element.value)?.focus();document.addEventListener('keydown',keydown)}}
  else{document.removeEventListener('keydown',keydown);if(returnTo?.isConnected)returnTo.focus()}
 })
 onBeforeUnmount(()=>document.removeEventListener('keydown',keydown))
}
