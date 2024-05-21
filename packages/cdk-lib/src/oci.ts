import { TemplateTag, TemplateTransformer } from "common-tags";

/**
 * Format of the OCI reference string.
 */
const OCI_PATTERN = /([\w._-]+\/)?([\w._-]+\/)?[\w._-]+:.+/;

/**
 * No-op transform that makes sure OCI reference string is of the correct
 * format and removes the 'oci:' prefix.
 */
const OCITransform: TemplateTransformer = {
  onEndResult(spec) {
    if (!OCI_PATTERN.test(spec)) {
      throw new Error(`Invalid OCI reference`);
    }
    return spec;
  },
};

/**
 * Converts a reference string to an OCI container to an image name
 * and tag usable by kubernetes. This is so that dependency management
 * tools like Renovate can detect and update container versions.
 */
export const oci = new TemplateTag(OCITransform);
