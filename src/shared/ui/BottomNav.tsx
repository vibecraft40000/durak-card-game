import { NavLink } from "react-router-dom";
import { PlayIcon, PlusIcon, UserIcon } from "@/shared/ui/Icons";

const NAV_ITEMS = [
  { to: "/play", label: "Играть", icon: PlayIcon, end: true },
  { to: "/play/create", label: "Создать игру", icon: PlusIcon, end: false },
  { to: "/profile", label: "Профиль", icon: UserIcon, end: false },
];

export function BottomNav() {
  return (
    <nav className="bottom-nav bottom-nav--three">
      {NAV_ITEMS.map((item) => (
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
