import { NavLink } from "react-router-dom";
import { PlayIcon, PlusIcon, UserIcon } from "@/shared/ui/Icons";

const NAV_ITEMS = [
  { to: "/play", label: "Играть", icon: PlayIcon },
  { to: "/create", label: "Создать игру", icon: PlusIcon },
  { to: "/profile", label: "Профиль", icon: UserIcon },
];

export function BottomNav() {
  return (
    <nav className="bottom-nav bottom-nav--three">
      {NAV_ITEMS.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          className={({ isActive }) => `bottom-nav__item ${isActive ? "bottom-nav__item--active" : ""}`}
        >
          <item.icon size={20} />
          <span>{item.label}</span>
        </NavLink>
      ))}
    </nav>
  );
}
