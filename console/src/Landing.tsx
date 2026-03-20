import { useRef } from 'react';
import {
  Activity,
  ArrowRight,
  Cpu,
  Database,
  GitBranch,
  Layers,
  Network,
  Shield,
  Zap,
  Users,
} from 'lucide-react';
import { motion, useScroll, useTransform } from 'framer-motion';

interface LandingProps {
  onGetStarted: () => void;
  onLogin: () => void;
  onDocs: () => void;
}

const proofPoints = [
  'HIPAA-compliant causal invalidation',
  'SHA-256 verified audit trails',
  'AES-256 encryption at rest',
];

const featureCards = [
  {
    icon: Shield,
    title: 'Healthcare-Grade Compliance',
    description: 'Audit logs with SHA-256 verification and AES-256 encryption at rest, built for HIPAA and SOC2 compliance from day one.',
  },
  {
    icon: Users,
    title: 'Regulated Multi-Tenancy',
    description: 'Strict organization isolation and role-based access for complex healthcare enterprise deployments.',
  },
  {
    icon: Zap,
    title: 'Causal Invalidation',
    description: 'Automatically revoke PHI access and clinical reasoning the moment patient consent or lab data is retracted.',
  },
  {
    icon: Activity,
    title: 'Live reasoning audit',
    description: 'Inspect a session, replay the journal, and see what changed when a clinical premise failed.',
  },
  {
    icon: GitBranch,
    title: 'Justification graph',
    description: 'Support chains stay explicit instead of implied in prompt history, providing clear clinical provenance.',
  },
  {
    icon: Cpu,
    title: 'Agent-sidecar ergonomics',
    description: 'Drop the adapter into an existing medical workflow without rebuilding your application.',
  },
];

const useCases = [
  {
    title: 'Clinical Consent Management',
    detail: 'Propagate consent status through agentic reasoning. If a patient withdraws consent, all downstream record access and processing beliefs collapse instantly.',
  },
  {
    title: 'Lab Value Recalculation',
    detail: 'Medical agents often act on preliminary labs. When final, corrected values arrive, Velarix invalidates the stale conclusions.',
  },
  {
    title: 'Policy & Guideline Audits',
    detail: 'Provide a cryptographic provenance trail for every clinical decision made by an autonomous system.',
  },
];

function ExplodedViewHero() {
  const containerRef = useRef<HTMLDivElement>(null);
  const { scrollYProgress } = useScroll({
    target: containerRef,
    offset: ["start end", "end start"]
  });

  // Parallax shifts based on scroll
  const topLayerY = useTransform(scrollYProgress, [0, 1], [-40, 40]);
  const bottomLayerY = useTransform(scrollYProgress, [0, 1], [40, -40]);

  return (
    <div ref={containerRef} className="relative mx-auto w-full max-w-[640px] aspect-square flex items-center justify-center [perspective:2000px]">
      <motion.div 
        className="relative w-full h-full preserve-3d"
        initial={{ rotateX: 20, rotateY: -20 }}
        animate={{ rotateX: [20, 22, 20], rotateY: [-20, -18, -20] }}
        transition={{ duration: 8, repeat: Infinity, ease: "easeInOut" }}
      >
        {/* Top Layer: Agent Application */}
        <motion.div
          className="absolute inset-0 flex items-center justify-center z-30"
          style={{ y: topLayerY }}
          animate={{ x: [-15, -25, -15], y: [-65, -75, -65] }}
          transition={{ duration: 5, repeat: Infinity, repeatType: "mirror", ease: "easeInOut" }}
        >
          <div className="w-64 h-40 bg-[#0ea5e9]/10 border border-[#0ea5e9]/30 rounded-2xl backdrop-blur-md shadow-2xl flex flex-col p-4">
             <div className="flex items-center gap-2 mb-3">
               <div className="w-3 h-3 rounded-full bg-sky-400 shadow-[0_0_8px_rgba(56,189,248,0.8)]" />
               <span className="text-[0.6rem] uppercase tracking-widest text-sky-300 font-bold">Healthcare Agent</span>
             </div>
             <div className="space-y-2">
               <div className="h-2 w-full bg-sky-400/20 rounded" />
               <div className="h-2 w-3/4 bg-sky-400/20 rounded" />
               <div className="h-2 w-1/2 bg-sky-400/20 rounded" />
             </div>
          </div>
        </motion.div>

        {/* Middle Layer: Velarix Middleware */}
        <motion.div
          className="absolute inset-0 flex items-center justify-center z-20"
          animate={{ y: [0, -5, 0], x: [0, 5, 0] }}
          transition={{ duration: 6, repeat: Infinity, repeatType: "mirror", ease: "easeInOut", delay: 0.5 }}
        >
          <div className="w-80 h-48 bg-[#050b16] border border-sky-400/20 rounded-2xl shadow-[0_20px_60px_rgba(0,0,0,0.6)] flex flex-col p-5 overflow-hidden relative">
            <div className="absolute inset-0 bg-[linear-gradient(rgba(56,189,248,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(56,189,248,0.03)_1px,transparent_1px)] bg-[size:20px_20px]" />
            <div className="relative z-10">
              <div className="flex items-center justify-between mb-4">
                <span className="text-[0.6rem] uppercase tracking-widest text-sky-400 font-bold">Velarix Kernel</span>
                <div className="flex gap-1">
                  <div className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                  <div className="w-1.5 h-1.5 rounded-full bg-amber-400" />
                </div>
              </div>
              <div className="flex justify-center gap-4 mt-2">
                 <div className="w-12 h-12 rounded-xl border border-sky-400/30 bg-sky-400/5 flex items-center justify-center">
                   <Network className="h-6 w-6 text-sky-400" />
                 </div>
                 <div className="w-12 h-12 rounded-xl border border-sky-400/30 bg-sky-400/5 flex items-center justify-center">
                   <Shield className="h-6 w-6 text-sky-400" />
                 </div>
                 <div className="w-12 h-12 rounded-xl border border-sky-400/30 bg-sky-400/5 flex items-center justify-center">
                   <Activity className="h-6 w-6 text-sky-400" />
                 </div>
              </div>
              <motion.div 
                className="mt-6 h-1 bg-sky-400/30 rounded-full overflow-hidden"
                animate={{ width: ["0%", "100%", "0%"] }}
                transition={{ duration: 3, repeat: Infinity, ease: "easeInOut" }}
              >
                <div className="h-full bg-sky-400 w-1/3" />
              </motion.div>
            </div>
          </div>
          {/* Dynamic Shadow between middle and bottom */}
          <motion.div 
            className="absolute -bottom-12 w-72 h-16 bg-sky-900/30 blur-2xl rounded-full"
            animate={{ opacity: [0.2, 0.5, 0.2], scale: [0.8, 1.1, 0.8] }}
            transition={{ duration: 5, repeat: Infinity, repeatType: "mirror" }}
          />
        </motion.div>

        {/* Bottom Layer: Data/Core Engine */}
        <motion.div
          className="absolute inset-0 flex items-center justify-center z-10"
          style={{ y: bottomLayerY }}
          animate={{ x: [20, 30, 20], y: [60, 70, 60] }}
          transition={{ duration: 5, repeat: Infinity, repeatType: "mirror", ease: "easeInOut", delay: 1 }}
        >
          <div className="w-72 h-44 bg-slate-900/40 border border-white/5 rounded-2xl backdrop-blur-sm flex flex-col p-4 opacity-70">
             <div className="text-[0.6rem] uppercase tracking-widest text-slate-500 mb-3 font-bold">Epistemic Storage</div>
             <div className="grid grid-cols-4 gap-2">
               {[...Array(12)].map((_, i) => (
                 <div key={i} className="h-6 bg-slate-800/50 rounded" />
               ))}
             </div>
          </div>
        </motion.div>
      </motion.div>
    </div>
  );
}

export function Landing({ onGetStarted, onLogin, onDocs }: LandingProps) {
  return (
    <div className="page-backdrop min-h-screen text-slate-100 selection:bg-sky-500/30 overflow-x-hidden">
      <nav className="fixed inset-x-0 top-0 z-50 border-b border-white/6 bg-[#050913]/78 backdrop-blur-2xl">
        <div className="mx-auto flex h-20 max-w-7xl items-center justify-between px-6 lg:px-8">
          <motion.div 
            initial={{ opacity: 0, x: -20 }}
            animate={{ opacity: 1, x: 0 }}
            className="flex items-center gap-3"
          >
            <div className="flex h-11 w-11 items-center justify-center rounded-2xl border border-sky-300/20 bg-sky-400/10 shadow-[0_10px_30px_rgba(14,165,233,0.15)]">
              <Database className="h-5 w-5 text-sky-300" />
            </div>
            <div>
              <div className="font-display text-xl font-bold tracking-tight text-white">Velarix</div>
              <div className="text-[0.65rem] uppercase tracking-[0.28em] text-slate-500">Regulated State Layer</div>
            </div>
          </motion.div>

          <div className="hidden items-center gap-8 text-sm text-slate-400 md:flex">
            <button onClick={onDocs} className="transition-colors hover:text-white font-medium">Documentation</button>
            <button onClick={onLogin} className="transition-colors hover:text-white font-medium">Console login</button>
            <motion.button 
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              onClick={onGetStarted} 
              className="button-primary px-5 py-3 text-sm font-semibold shadow-lg shadow-sky-500/20"
            >
              Get API key <ArrowRight className="h-4 w-4" />
            </motion.button>
          </div>
        </div>
      </nav>

      <main>
        <section className="relative overflow-hidden px-6 pb-24 pt-36 lg:px-8 lg:pb-28 lg:pt-44">
          <div className="absolute inset-x-0 top-24 h-72 bg-[radial-gradient(circle_at_center,rgba(56,189,248,0.18),transparent_62%)] blur-3xl" />
          <div className="mx-auto grid max-w-7xl items-center gap-16 lg:grid-cols-[1.1fr_0.9fr]">
            <motion.div 
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6 }}
              className="relative z-10"
            >
              <div className="section-kicker shadow-inner">
                <Shield className="h-3.5 w-3.5" /> HIPAA & SOC2 Compliant
              </div>
              <h1 className="font-display mt-8 max-w-4xl text-5xl font-bold leading-[0.92] tracking-[-0.05em] text-white sm:text-6xl lg:text-7xl xl:text-[5.2rem] drop-shadow-sm">
                The trust layer for Healthcare AI.
              </h1>
              <p className="mt-8 max-w-2xl text-lg leading-8 text-slate-300 lg:text-xl">
                Velarix is the deterministic state layer for regulated AI: a reasoning boundary that invalidates stale downstream medical beliefs the moment a patient premise changes.
              </p>

              <div className="mt-10 flex flex-col gap-4 sm:flex-row">
                <motion.button 
                  whileHover={{ scale: 1.05 }}
                  whileTap={{ scale: 0.95 }}
                  onClick={onGetStarted} 
                  className="button-primary px-6 py-4 text-base font-semibold shadow-xl shadow-sky-500/20"
                >
                  Start with the console <ArrowRight className="h-5 w-5" />
                </motion.button>
                <motion.button 
                  whileHover={{ scale: 1.05 }}
                  whileTap={{ scale: 0.95 }}
                  onClick={onDocs} 
                  className="button-secondary px-6 py-4 text-base font-semibold shadow-lg shadow-black/20"
                >
                  Review healthcare docs
                </motion.button>
              </div>

              <div className="mt-12 grid gap-3 sm:grid-cols-3">
                {proofPoints.map((point) => (
                  <motion.div 
                    key={point} 
                    whileHover={{ y: -2 }}
                    className="rounded-2xl border border-white/8 bg-white/[0.03] px-4 py-3 text-sm text-slate-300 flex items-center gap-2 hover:bg-white/[0.05] transition-colors"
                  >
                    <Zap className="h-3.5 w-3.5 text-sky-400" /> {point}
                  </motion.div>
                ))}
              </div>
            </motion.div>

            <ExplodedViewHero />
          </div>
        </section>

        <section className="px-6 py-24 lg:px-8 border-t border-white/5 bg-[#030712]/50">
          <div className="mx-auto max-w-7xl">
            <div className="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
              <div>
                <div className="section-kicker">
                  <Layers className="h-3.5 w-3.5" /> Product capabilities
                </div>
                <h2 className="font-display mt-6 text-4xl font-bold tracking-[-0.04em] text-white sm:text-5xl">
                   Built for the clinical control plane.
                </h2>
              </div>
            </div>

            <div className="mt-12 grid gap-6 md:grid-cols-2 xl:grid-cols-3">
              {featureCards.map((card, i) => (
                <FeatureCard key={card.title} delay={i * 0.1} {...card} />
              ))}
            </div>
          </div>
        </section>

        <section className="px-6 py-24 lg:px-8 border-t border-white/5 relative">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_bottom_left,rgba(56,189,248,0.05),transparent_40%)]" />
          <div className="surface-panel mx-auto max-w-7xl rounded-[2.5rem] p-10 md:p-12 lg:p-16 relative overflow-hidden shadow-2xl">
            <div className="grid gap-10 lg:grid-cols-[0.85fr_1.15fr] relative z-10">
              <div>
                <div className="section-kicker">
                  <Activity className="h-3.5 w-3.5" /> Clinical Use Cases
                </div>
                <h2 className="font-display mt-6 text-4xl font-bold tracking-[-0.04em] text-white">
                  Deterministic reasoning where it matters most.
                </h2>
              </div>
              <div className="grid gap-6 md:grid-cols-1">
                {useCases.map((useCase, index) => (
                  <motion.div 
                    key={useCase.title} 
                    initial={{ opacity: 0, x: 20 }}
                    whileInView={{ opacity: 1, x: 0 }}
                    viewport={{ once: true, margin: "-100px" }}
                    transition={{ delay: index * 0.1 }}
                    whileHover={{ scale: 1.02 }}
                    className="group rounded-3xl border border-white/8 bg-black/20 p-8 hover:border-sky-400/30 transition-all shadow-lg hover:shadow-sky-900/20"
                  >
                    <div className="text-[0.72rem] uppercase tracking-[0.22em] text-sky-400 font-bold mb-4">Wedge 0{index + 1}</div>
                    <div className="font-display text-2xl font-semibold text-white group-hover:text-sky-100 transition-colors">{useCase.title}</div>
                    <p className="mt-3 text-base leading-7 text-slate-300">{useCase.detail}</p>
                  </motion.div>
                ))}
              </div>
            </div>
          </div>
        </section>
      </main>
    </div>
  );
}

function FeatureCard({
  icon: Icon,
  title,
  description,
  delay
}: {
  icon: typeof Database;
  title: string;
  description: string;
  delay: number;
}) {
  return (
    <motion.div 
      initial={{ opacity: 0, y: 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-50px" }}
      transition={{ delay }}
      whileHover={{ y: -8, scale: 1.02 }}
      className="surface-panel group rounded-[2rem] p-8 relative overflow-hidden transition-all duration-300 shadow-lg hover:shadow-2xl hover:shadow-sky-900/20 hover:border-sky-500/20"
    >
      <div className="absolute inset-0 bg-gradient-to-br from-sky-400/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500" />
      <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-sky-300/14 bg-sky-400/10 mb-8 group-hover:scale-110 transition-transform duration-300 shadow-inner">
        <Icon className="h-6 w-6 text-sky-300" />
      </div>
      <div className="font-display text-2xl font-semibold tracking-tight text-white group-hover:text-sky-50 transition-colors">{title}</div>
      <p className="mt-4 text-sm leading-7 text-slate-300">{description}</p>
    </motion.div>
  );
}
