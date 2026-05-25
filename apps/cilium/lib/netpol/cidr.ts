const RFC1918_CIDRS = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"];

export const cidr = {
  rfc1918(): string[] {
    return [...RFC1918_CIDRS];
  },
};
