import { useState, useEffect, useRef } from 'react';
import { Outlet, Link, useLocation, useNavigate } from 'react-router-dom';
import { Activity, LayoutDashboard, Settings, LogOut, Search, Menu, X, Brain } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

export default function Layout() {
  const navigate = useNavigate();
  const location = useLocation();
  const [isProfileOpen, setIsProfileOpen] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const profileRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (profileRef.current && !profileRef.current.contains(e.target as Node)) {
        setIsProfileOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Close mobile menu on route change
  useEffect(() => {
    setIsMobileMenuOpen(false);
  }, [location.pathname]);

  const handleLogout = () => {
    localStorage.removeItem('velarix_token');
    navigate('/');
  };

  const navItems = [
    { path: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
    { path: '/graph', label: 'Neural Graph', icon: Activity },
    { path: '/explanations', label: 'Explanations', icon: Brain },
    { path: '/settings', label: 'Settings', icon: Settings },
  ];

  const sidebarContent = (
    <>
      <div className="h-16 flex items-center px-6 border-b border-v-border">
        <Link to="/" className="flex items-center">
          <div className="w-6 h-6 rounded bg-gradient-to-br from-v-accent to-v-success flex items-center justify-center mr-3">
            <span className="text-v-bg font-bold font-display text-xs">V</span>
          </div>
          <span className="text-white font-display font-medium text-lg tracking-wide">Velarix</span>
        </Link>
      </div>
      
      <div className="p-4 flex-1">
        <div className="text-xs font-mono text-v-text-muted mb-4 px-2 uppercase tracking-wider">Workspace</div>
        <nav className="space-y-1">
          {navItems.map((item) => {
            const isActive = location.pathname === item.path;
            const Icon = item.icon;
            return (
              <Link
                key={item.path}
                to={item.path}
                className={`flex items-center space-x-3 px-3 py-2 rounded-lg transition-colors relative group ${isActive ? 'text-white' : 'text-v-text-muted hover:text-white hover:bg-white/5'}`}
              >
                {isActive && (
                  <motion.div layoutId="activeNav" className="absolute left-0 w-1 h-full bg-v-accent rounded-r-md" />
                )}
                <Icon className="w-5 h-5" />
                <span className="font-medium text-sm">{item.label}</span>
              </Link>
            );
          })}
        </nav>
      </div>

      <div className="p-4 border-t border-v-border">
        <button 
          onClick={handleLogout}
          className="flex items-center space-x-3 text-v-text-muted hover:text-v-error transition-colors px-3 py-2 w-full rounded-lg hover:bg-v-error/10"
        >
          <LogOut className="w-5 h-5" />
          <span className="font-medium text-sm">Sign out</span>
        </button>
      </div>
    </>
  );

  return (
    <div className="flex h-screen bg-v-bg font-sans overflow-hidden">
      {/* Desktop Sidebar */}
      <aside className="hidden lg:flex w-64 border-r border-v-border bg-v-bg-card flex-col shrink-0">
        {sidebarContent}
      </aside>

      {/* Mobile Sidebar (Drawer) */}
      <AnimatePresence>
        {isMobileMenuOpen && (
          <>
            <motion.div 
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => setIsMobileMenuOpen(false)}
              className="fixed inset-0 bg-black/60 backdrop-blur-sm z-[60] lg:hidden"
            />
            <motion.aside
              initial={{ x: '-100%' }}
              animate={{ x: 0 }}
              exit={{ x: '-100%' }}
              transition={{ type: 'spring', damping: 25, stiffness: 200 }}
              className="fixed inset-y-0 left-0 w-72 bg-v-bg-card border-r border-v-border z-[70] flex flex-col lg:hidden"
            >
              <div className="absolute top-4 right-4 lg:hidden">
                <button 
                  onClick={() => setIsMobileMenuOpen(false)}
                  className="p-2 text-v-text-muted hover:text-white"
                >
                  <X className="w-6 h-6" />
                </button>
              </div>
              {sidebarContent}
            </motion.aside>
          </>
        )}
      </AnimatePresence>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0">
        <header className="h-16 border-b border-v-border bg-v-bg/50 backdrop-blur-md flex items-center justify-between px-4 md:px-8 z-10 sticky top-0">
          <div className="flex items-center space-x-4">
             <button 
               onClick={() => setIsMobileMenuOpen(true)}
               className="lg:hidden p-2 -ml-2 text-v-text-muted hover:text-white"
             >
               <Menu className="w-6 h-6" />
             </button>
             
             {/* Path breadcrumbs placeholder */}
             <span className="text-v-text-muted text-sm font-mono filter mix-blend-screen opacity-70 cursor-default hidden sm:inline-block">
               {location.pathname.substring(1).replace('/', ' / ')}
             </span>
          </div>
          
          <div className="flex items-center space-x-2 md:space-x-4">
            <div className="relative group hidden md:block">
              <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-v-text-muted group-focus-within:text-v-accent transition-colors" />
              <input 
                type="text" 
                placeholder="Search resources..." 
                className="bg-v-bg-card border border-v-border rounded-md pl-9 pr-4 py-1.5 text-sm focus:outline-none focus:border-v-accent text-white w-40 xl:w-64 transition-all focus:ring-1 focus:ring-v-accent/50"
              />
            </div>
            
            <button className="md:hidden p-2 text-v-text-muted hover:text-white">
              <Search className="w-5 h-5" />
            </button>

            <div className="relative" ref={profileRef}>
              <div 
                onClick={() => setIsProfileOpen(!isProfileOpen)}
                className="h-8 w-8 rounded-full bg-v-border flex items-center justify-center cursor-pointer border border-white/10 overflow-hidden hover:border-white/30 transition-colors"
               >
                 <img src="https://api.dicebear.com/7.x/avataaars/svg?seed=Felix" alt="User Avatar" />
              </div>
              
              <AnimatePresence>
                {isProfileOpen && (
                  <motion.div
                    initial={{ opacity: 0, y: 10, scale: 0.95 }}
                    animate={{ opacity: 1, y: 0, scale: 1 }}
                    exit={{ opacity: 0, y: 10, scale: 0.95 }}
                    className="absolute right-0 top-full mt-2 w-48 bg-[#0a0a0a] border border-white/10 rounded-xl shadow-2xl py-2 z-50 origin-top-right"
                  >
                    <div className="px-4 py-2 border-b border-white/10 mb-2">
                       <p className="text-sm font-medium text-white truncate">agent@velarix.io</p>
                       <p className="text-xs text-v-text-muted">Administrator</p>
                    </div>
                    <button 
                      onClick={handleLogout}
                      className="w-full text-left px-4 py-2 text-sm text-v-text-muted hover:text-white hover:bg-white/5 transition-colors flex items-center"
                    >
                      <LogOut className="w-4 h-4 mr-2" />
                      Sign out
                    </button>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </div>
        </header>
        
        <main className="flex-1 overflow-y-auto p-4 md:p-8 relative">
           <Outlet />
        </main>
      </div>
    </div>
  );
}
