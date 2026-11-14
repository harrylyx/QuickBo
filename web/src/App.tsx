import Nav from './components/Nav';
import Hero from './components/Hero';
import DemoSection from './components/DemoSection';
import Features from './components/Features';
import Sources from './components/Sources';
import Statement from './components/Statement';
import Install from './components/Install';
import Footer from './components/Footer';

export default function App() {
  return (
    <>
      <Nav />
      <main id="top">
        <Hero />
        <DemoSection />
        <Features />
        <Sources />
        <Statement />
        <Install />
      </main>
      <Footer />
    </>
  );
}
