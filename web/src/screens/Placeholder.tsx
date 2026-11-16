export function Placeholder({ title }: { title: string }) {
  return (
    <section className="screen">
      <h1>{title}</h1>
      <p className="soft">Not part of this build yet.</p>
    </section>
  );
}
