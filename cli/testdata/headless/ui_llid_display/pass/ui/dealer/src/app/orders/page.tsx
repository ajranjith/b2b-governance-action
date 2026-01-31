export default function Page() {
  fetch("/api/dealer/orders");
  return <div data-llid="x">TraceBadge</div>;
}
