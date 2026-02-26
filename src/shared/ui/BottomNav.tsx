import { NavLink } from "react-router-dom";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { PlayIcon, PlusIcon, UserIcon } from "@/shared/ui/Icons";

export function BottomNav() {
  const { t } = useLanguage();

  const navItems = [
    { to: "/play", label: t("nav.play"), icon: PlayIcon, end: true },
    { to: "/play/create", label: t("nav.createGame"), icon: PlusIcon, end: false },
    { to: "/profile", label: t("nav.profile"), icon: UserIcon, end: false },
  ];

  return (
    <nav className="bottom-nav bottom-nav--three">
      {navItems.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          className={({ isActive }) => `bottom-nav__item ${isActive ? "bottom-nav__item--active" : ""}`}
        >
          <item.icon size={20} />
          <span>{item.label}</span>
        </NavLink>
      ))}
    </nav>
  );
}
