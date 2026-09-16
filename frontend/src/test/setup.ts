// happy-dom (this project's Vitest test environment) does not implement the
// Pointer Events capture API or scrollIntoView. reka-ui (the Radix Vue port
// backing components/ui/select and friends) calls these unconditionally on
// pointerdown/keyboard navigation, so any test that opens a shadcn Select
// throws "target.hasPointerCapture is not a function" without this shim.
if (typeof Element !== 'undefined') {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false
  }
  if (!Element.prototype.setPointerCapture) {
    Element.prototype.setPointerCapture = () => {}
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {}
  }
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {}
  }
}
