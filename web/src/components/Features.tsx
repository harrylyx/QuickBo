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
          <h2 id="features-h">为多片源观看而设计</h2>
        </div>
        <div className="grid qb-feature-grid">
          {FEATURES.map((feature) => <Card feature={feature} key={feature.id} />)}
        </div>
      </div>
    </section>
  );
}
