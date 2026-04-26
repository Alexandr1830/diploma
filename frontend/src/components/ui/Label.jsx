// Минимальный Label, повторяющий API shadcn/ui: принимает `htmlFor`, `className`
// и любые дополнительные пропсы. Оформление задано классом .ui-label в App.css.
//
// Использование:
//   const id = useId()
//   <Label htmlFor={id}>Роль</Label>
//   <SelectNative id={id} ...>...</SelectNative>
export function Label({ htmlFor, className = '', children, ...rest }) {
  return (
    <label
      htmlFor={htmlFor}
      className={`ui-label ${className}`.trim()}
      {...rest}
    >
      {children}
    </label>
  )
}

export default Label
