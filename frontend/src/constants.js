// Geofence categories allowed by the backend, with a label and a map colour.
export const CATEGORIES = [
  { value: "delivery_zone", label: "Delivery zone", color: "#2563eb" },
  { value: "restricted_zone", label: "Restricted zone", color: "#dc2626" },
  { value: "toll_zone", label: "Toll zone", color: "#d97706" },
  { value: "customer_area", label: "Customer area", color: "#059669" },
];

export function categoryLabel(value) {
  const category = CATEGORIES.find((c) => c.value === value);
  return category ? category.label : value;
}

export function categoryColor(value) {
  const category = CATEGORIES.find((c) => c.value === value);
  return category ? category.color : "#6b7280";
}

// formatTime shows an ISO timestamp in the reader's own time zone.
export function formatTime(iso) {
  if (!iso) return "";
  return new Date(iso).toLocaleString(undefined, {
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

// localInputValue gives "now" in the form a datetime-local input expects (YYYY-MM-DDTHH:MM).
export function localInputValue(date = new Date()) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return local.toISOString().slice(0, 16);
}
