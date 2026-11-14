import { FEATURES } from '../data/features';
import FeatureVisual from './FeatureVisuals';

function Card({ feature }: { feature: (typeof FEATURES)[number] }) {
  return (
    <article className="card card--row card--open span-3">
      <h3>{feature.title}</h3>
      <p>{feature.body}</p>
      <div className="panel panel--inset">
        <div className="stage qb-feature-stage">
          <FeatureVisual kind={feature.visual} />
        </div>
      </div>
    </article>
  );
}

export default function Features() {
  return (
    <section className="section" id="features" aria-labelledby="features-h">
      <div className="wrap">
        <div className="section-head">
          <h2 id="features-h">看片时用得上</h2>
          <p className="t-lead muted">TVBox 和 FongMi 的订阅都能用。搜索、换线路、接着看，都在侧栏里。</p>
        </div>
        <div className="grid qb-feature-grid">
          {FEATURES.map((feature) => <Card feature={feature} key={feature.id} />)}
        </div>
      </div>
    </section>
  );
}
