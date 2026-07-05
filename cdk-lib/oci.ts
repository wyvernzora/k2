export function oci(strings: TemplateStringsArray, ...values: readonly never[]): string {
  if (values.length > 0) {
    throw new Error("OCI image references must be static");
  }
  return strings[0] ?? "";
}
