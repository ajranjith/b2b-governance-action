export default function Page() {
  fetch("/api/dealer/orders");
  return <div>Orders</div>;
}
