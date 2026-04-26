// SelectNative — одна точка стиля для всех <select> в проекте.
// Повторяет вид примера shadcn (border-transparent, no-shadow), но фон белый
// (как у card-поверхностей в нашей теме), без зависимости от Tailwind.
//
// Принимает все стандартные пропсы <select> + className для добавочных классов.
// Иконка-шеврон рисуется через background-image (см. .select-native в App.css),
// поэтому padding-right учитывает её ширину.
export function SelectNative({ id, className = '', children, ...rest }) {
  return (
    <select
      id={id}
      className={`select-native ${className}`.trim()}
      {...rest}
    >
      {children}
    </select>
  )
}

export default SelectNative
